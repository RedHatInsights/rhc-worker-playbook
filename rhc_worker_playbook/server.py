import sys
import os
from .constants import WORKER_LIB_DIR
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
try:
    # TODO: try other eggs
    sys.path.append("/var/lib/insights/last_stable.egg")
    from insights.client.core.apps.ansible.playbook_verifier import verify
except ImportError as e:
    print("Could not import insights-core: %s" % e)
    def verify(playbook):
        print("WARNING: Playbook verification disabled.")
        return playbook

YGG_SOCKET_ADDR = os.environ.get('YGG_SOCKET_ADDR')
if not YGG_SOCKET_ADDR:
    print("Missing YGG_SOCKET_ADDR environment variable")
    sys.exit(1)
# massage the value for python grpc
YGG_SOCKET_ADDR = YGG_SOCKET_ADDR.replace("unix:@", "unix-abstract:")

def generateRequest(events, return_url):
    # TODO?: generate by hand so request isn't a dependency
    # TODO: change the content type to what it should be
    return Request('POST', return_url, files={
        "file": ("runner-events", json.dumps(events), "application/vnd.redhat.advisor.collection+tgz"),
        "metadata": "{}"
    }).prepare()

def parseFailure(event):
    # generate the error code and details from the failure event
    errorCode = "UNDEFINED_ERROR"
    errorDetails = event.get('stdout')
    if "The command was not found or was not executable: ansible-playbook" in errorDetails:
        errorCode = "ANSIBLE_PLAYBOOK_NOT_INSTALLED"
    # TODO: enumerate more failure types
    return errorCode, errorDetails

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

        events = []
        # parse playbook from data field
        try:
            # required fields
            playbook_str = request.content
            response_to = request.message_id
            crc_dispatcher_correlation_id = request.metadata.get('crc_dispatcher_correlation_id')
            response_interval = request.metadata.get('response_interval')
            return_url = request.metadata.get('return_url')
        except LookupError as e:
            print("Missing attribute in message: %s" % e)

        playbook = yaml.safe_load(playbook_str.decode('utf-8'))

        # call insights-core lib to verify playbook
        playbook = verify(playbook)

        # required event for cloud connector
        on_start = executor_on_start(correlation_id=crc_dispatcher_correlation_id)
        events.append(on_start)

        # run playbook
        # TODO: use async and poll the thread on the "response interval" timer
        runner = ansible_runner.interface.run(
            playbook=playbook,
            envvars={"PYTHONPATH": WORKER_LIB_DIR},
            quiet=True)

        for evt in runner.events:
            events.append(evt)

        if runner.status == 'failed':
            # last event sould be the failure, find the reason
            errorCode, errorDetails = parseFailure(events[-1])
            if errorCode == "ANSIBLE_PLAYBOOK_NOT_INSTALLED":
                # TODO: send message back to dispatcher w/ the failure
                print("The rhc-worker-playbook package requires the ansible package to be installed.")
            # required event for cloud connector
            on_failed = executor_on_failed(correlation_id=crc_dispatcher_correlation_id, error_code=errorCode, error_details=errorDetails)
            events.append(on_failed)

        print(events)

        # generate the request body to ingress (form data)
        req = generateRequest(events, return_url)

        returnedEvents = yggdrasil_pb2.Data(
            message_id=str(uuid.uuid4()).encode('utf-8'),
            content=req.body,
            directive=return_url,
            metadata=req.headers,
            response_to=response_to)
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

