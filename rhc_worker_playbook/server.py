import sys
import os
from .constants import WORKER_LIB_DIR, STABLE_EGG, RPM_EGG
sys.path.append(WORKER_LIB_DIR)
import yaml
import grpc
import ansible_runner
import time
import json
import uuid
from requests import Request
from concurrent import futures
from .protocol import yggdrasil_pb2_grpc, yggdrasil_pb2
from .dispatcher_events import executor_on_start, executor_on_failed

for egg in (STABLE_EGG, RPM_EGG):
    # prefer stable > rpm
    try:
        if not os.path.exists(egg):
            raise ImportError("Egg %s is unavailable" % egg)
        sys.path.append(egg)
        from insights.client.apps.ansible.playbook_verifier import verify
        from insights.client.apps.ansible.playbook_verifier.contrib import oyaml
        VERIFY_ENABLED = True
        break
    except ImportError as e:
        print("Could not import insights-core: %s" % e)
        VERIFY_ENABLED = False

def _str2bool(_var):
    '''
    Python shenanigans to convert an env var string into a boolean
    '''
    if type(_var) == bool:
        return _var

    elif type(_var) == str:
        if _var.lower() == 'false':
            return False
        elif _var.lower() == 'true':
            return True
        else:
            print("Unknown boolean value %s, defaulting to True" % _var)
            return True

VERIFY_ENABLED = _str2bool(os.environ.get('YGG_VERIFY_PLAYBOOK', VERIFY_ENABLED))
VERIFY_VERSION_CHECK = _str2bool(os.environ.get('YGG_VERIFY_VERSION_CHECK', True))
YGG_SOCKET_ADDR = os.environ.get('YGG_SOCKET_ADDR')
if not YGG_SOCKET_ADDR:
    print("Missing YGG_SOCKET_ADDR environment variable")
    sys.exit(1)
# massage the value for python grpc
YGG_SOCKET_ADDR = YGG_SOCKET_ADDR.replace("unix:@", "unix-abstract:")

def _newlineDelimited(events):
    '''
    Dump a list into a newline-delimited JSON format
    '''
    output = ''
    for e in events:
        output += json.dumps(e) + '\n'
    return output

def _generateRequest(events, return_url):
    '''
    Generate the HTTP request
    '''
    # TODO?: generate by hand so request isn't a dependency
    # TODO: change the content type to what it should be
    return Request('POST', return_url, files={
        "file": ("runner-events", _newlineDelimited(events), "application/vnd.redhat.playbook.v1+jsonl"),
        "metadata": "{}"
    }).prepare()

def _composeDispatcherMessage(events, return_url, response_to):
    '''
    Create the message with event data to send back to Dispatcher
    '''
    req = _generateRequest(events, return_url)
    return yggdrasil_pb2.Data(
        message_id=str(uuid.uuid4()).encode('utf-8'),
        content=req.body,
        directive=return_url,
        metadata=req.headers,
        response_to=response_to)

def _parseFailure(event):
    '''
    Generate the error code and details from the failure event
    '''
    errorCode = "UNDEFINED_ERROR"
    errorDetails = event.get('stdout')
    if "The command was not found or was not executable: ansible-playbook" in errorDetails:
        errorCode = "ANSIBLE_PLAYBOOK_NOT_INSTALLED"
    # TODO: enumerate more failure types
    return errorCode, errorDetails

class Events(list):
    '''
    Extension of list to receive ansible-runner events
    '''
    def __init__(self):
        pass

    def addEvent(self, event_data):
        self.append(event_data)


class WorkerService(yggdrasil_pb2_grpc.WorkerServicer):

    def __init__(self, *args, **kwargs):
        dispatcher = kwargs.get('dispatcher', None)
        if not dispatcher:
            raise ValueError('No dispatcher parameter was provided to the WorkerService')
        self.dispatcher = dispatcher

    def Send(self, request, context):
        '''
        Act on messages sent to the WorkerService
        '''
        # we have received it
        yggdrasil_pb2.Receipt()

        events = Events()
        # parse playbook from data field
        try:
            # required fields
            playbook_str = request.content
            response_to = request.message_id
            crc_dispatcher_correlation_id = request.metadata.get('crc_dispatcher_correlation_id')
            response_interval = float(request.metadata.get('response_interval'))
            return_url = request.metadata.get('return_url')
        except LookupError as e:
            print("Missing attribute in message: %s" % e)

        if VERIFY_ENABLED:
            playbook = oyaml.load(playbook_str.decode('utf-8'))
            # call insights-core lib to verify playbook
            # don't catch exception - allow it to bubble up to rhcd
            playbook = verify(playbook, checkVersion=VERIFY_VERSION_CHECK)
        else:
            print("WARNING: Playbook verification disabled.")
            playbook = yaml.safe_load(playbook_str.decode('utf-8'))

        for item in playbook:
            if 'vars' in item:
                # remove signature field, ansible-runner dislikes bytes
                item['vars'].pop('insights_signature', None)

        # required event for cloud connector
        on_start = executor_on_start(correlation_id=crc_dispatcher_correlation_id)
        events.addEvent(on_start)

        # run playbook
        runnerThread, runner = ansible_runner.interface.run_async(
            playbook=playbook,
            envvars={"PYTHONPATH": WORKER_LIB_DIR},
            event_handler=events.addEvent,
            quiet=True)

        # initialize elapsed counter
        elapsedTime = 0
        startTime = time.time()
        while runnerThread.is_alive():
            elapsedTime = time.time() - startTime
            if elapsedTime >= response_interval:
                # hit the interval, post events
                returnedEvents = _composeDispatcherMessage(events, return_url, response_to)
                response = self.dispatcher.Send(returnedEvents)
                # reset interval timer
                elapsedTime = 0
                startTime = time.time()

        if runner.status == 'failed':
            # last event sould be the failure, find the reason
            errorCode, errorDetails = _parseFailure(events[-1])
            if errorCode == "ANSIBLE_PLAYBOOK_NOT_INSTALLED":
                # TODO: send message back to dispatcher w/ the failure
                print("The rhc-worker-playbook package requires the ansible package to be installed.")
            # required event for cloud connector
            on_failed = executor_on_failed(correlation_id=crc_dispatcher_correlation_id, error_code=errorCode, error_details=errorDetails)
            events.addEvent(on_failed)

        # send the final message after playbook completed
        returnedEvents = _composeDispatcherMessage(events, return_url, response_to)
        response = self.dispatcher.Send(returnedEvents)
        return

def serve():
    # open the channel to ygg Dispatcher
    channel = grpc.insecure_channel(YGG_SOCKET_ADDR)
    dispatcher = yggdrasil_pb2_grpc.DispatcherStub(channel)
    registrationResponse = dispatcher.Register(
        yggdrasil_pb2.RegistrationRequest(
            handler="rhc-worker-playbook",
            detached_content=True,
            pid=os.getpid()))
    registered = registrationResponse.registered
    if not registered:
        print("Could not register rhc-worker-playbook.")
        sys.exit(1)
    address = registrationResponse.address.replace("@", "unix-abstract:")

    # create server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    try:
        yggdrasil_pb2_grpc.add_WorkerServicer_to_server(WorkerService(dispatcher=dispatcher), server)
    except ValueError as e:
        print(e)
        sys.exit(1)
    server.add_insecure_port(address)

    # off to the races
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()

