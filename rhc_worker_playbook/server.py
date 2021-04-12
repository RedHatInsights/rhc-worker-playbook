import sys
import os
from .constants import WORKER_LIB_DIR, STABLE_EGG, RPM_EGG, CONFIG_FILE
sys.path.insert(0, WORKER_LIB_DIR)
import toml
import yaml
import grpc
import ansible_runner
import time
import json
import uuid
import copy
import subprocess
from requests import Request
from concurrent import futures
from .protocol import yggdrasil_pb2_grpc, yggdrasil_pb2
from .dispatcher_events import executor_on_start, executor_on_failed

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

def _loadConfig():
    # load config file
    _config = {}
    if os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE) as _f:
            _config = toml.load(_f)
    else:
        print("WARNING: Config file does not exist: %s. Using defaults." % CONFIG_FILE)

    parsedConfig = {
        "verify_enabled": _config.get('verify_playbook', True),
        "verify_version_check": _config.get('verify_playbook_version_check', True),
        "insights_core_gpg_check": _config.get('insights_core_gpg_check', True)
    }
    return parsedConfig

def _updateCore(config):
    '''
    Run the insights-client "update" phase alone to populate newest.egg
    '''
    env = {"PATH": ""}
    if config['insights_core_gpg_check'] == False:
        env["INSIGHTS_CORE_GPG_CHECK"] = "False"
    print(env)
    updateProc = subprocess.Popen(
        [sys.executable, os.path.join(os.path.dirname(__file__), "core_update.py")],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE)
    stdout, stderr = updateProc.communicate()
    print(stdout)
    print(stderr)
    if updateProc.returncode != 0:
        print("Could not perform insights-core update")

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

        # load configuration
        config = _loadConfig()

        # try to update insights-core
        _updateCore(config)

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
            # raise exception to bubble up to rhcd
            raise Exception("Missing attribute in message: %s" % e)
        
        if config["verify_enabled"]:
            args = ["insights-client", "--offline", "-m", "insights.client.apps.ansible.playbook_verifier", "--quiet"]
            env = {"PATH": ""}
            if config["insights_core_gpg_check"] == False:
                args.append("--no-gpg")
                env["BYPASS_GPG"] = "True"
            verifyProc = subprocess.Popen(
                args,
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            playbook_str, err = verifyProc.communicate(input=playbook_str)
            
            if err:
                print("WARNING: Unable to verify playbook")
            if verifyProc.returncode != 0:
                raise Exception("Unable to verify playbook: %s" % err)
            # remove this after insights-core fix
            stripped_pb = playbook_str.decode('utf-8').split("\n", 1)[1]
            playbook = yaml.safe_load(stripped_pb)
            print("Playbook verified")
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


