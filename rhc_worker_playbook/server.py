import sys
import os
from .constants import WORKER_LIB_DIR, CONFIG_FILE
sys.path.insert(0, WORKER_LIB_DIR)
import toml
import yaml
import grpc
import ansible_runner
import time
import json
import uuid
import atexit
from threading import Thread
from subprocess import Popen, PIPE
from requests import Request
from concurrent import futures
from .protocol import yggdrasil_pb2_grpc, yggdrasil_pb2
from .dispatcher_events import executor_on_start, executor_on_failed

# unbuffered stdout for logging to rhc
sys.stdout = os.fdopen(sys.stdout.fileno(), 'wb', buffering=0)
atexit.register(sys.stdout.close)

def _log(message):
    '''
    Send message as bytes over unbuffered stdout for
    RHC to log
    '''
    sys.stdout.write((message + '\n').encode())

# for some reason, without a short delay, RHC connects too quickly
#   and cloud connector service can't see it
time.sleep(5)

YGG_SOCKET_ADDR = os.environ.get('YGG_SOCKET_ADDR')
if not YGG_SOCKET_ADDR:
    _log("Missing YGG_SOCKET_ADDR environment variable")
    sys.exit(1)
# massage the value for python grpc
YGG_SOCKET_ADDR = YGG_SOCKET_ADDR.replace("unix:@", "unix-abstract:")
BASIC_PATH = "/sbin:/bin:/usr/sbin:/usr/bin"

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
        _log("WARNING: Config file does not exist: %s. Using defaults." % CONFIG_FILE)

    parsedConfig = {
        "directive": _config.get("directive", "rhc-worker-playbook"),
        "verify_playbook": _config.get("verify_playbook", True),
        "verify_playbook_version_check": _config.get("verify_playbook_version_check", True),
        "insights_core_gpg_check": _config.get("insights_core_gpg_check", True)
    }
    return parsedConfig


class Events(list):
    '''
    Extension of list to receive ansible-runner events
    '''
    def __init__(self):
        pass

    def addEvent(self, event):
        try:
            event.get("event_data", {}).get("res", {}).pop("ansible_facts", None)
        except AttributeError:
            # one of the fields was null/empty
            pass
        self.append(event)


class WorkerService(yggdrasil_pb2_grpc.WorkerServicer):

    def __init__(self, *args, **kwargs):
        dispatcher = kwargs.get('dispatcher', None)
        if not dispatcher:
            _log('No dispatcher parameter was provided to the WorkerService')
            raise Exception
        self.dispatcher = dispatcher

    def Send(self, request, context):
        '''
        Act on messages sent to the WorkerService
        '''
        # start the worker thread
        worker_thread = Thread(target=self._do_work, args=(request, context))
        worker_thread.start()
        # let them know we have received it
        return yggdrasil_pb2.Receipt()

    def _do_work(self, request, context):
        '''
        Do the actual work of calling ansible and everything else
        '''
        # load configuration
        config = _loadConfig()

        events = Events()
        # parse playbook from data field
        try:
            # required fields
            playbook_str = request.content
            response_to = request.message_id
            crc_dispatcher_correlation_id = request.metadata.get('crc_dispatcher_correlation_id')
            response_interval = request.metadata.get('response_interval')
            return_url = request.metadata.get('return_url')
        except LookupError as e:
            _log("ERROR: Missing attribute in message: %s" % e)
            raise

        if not crc_dispatcher_correlation_id:
            _log("WARNING: No crc_dispatcher_correlation_id")

        if not return_url:
            _log("WARNING: No return_url")

        if not response_interval:
            _log("WARNING: No response_interval. Defaulting to 300")
            response_interval = 300
        else:
            try:
                response_interval = float(response_interval)
            except (TypeError, ValueError) as e:
                _log("ERROR: Invalid response_interval")
                _log(str(e))
                raise

        if config["verify_playbook"]:
            _log("Verifying playbook...")
            # --payload here will be a no-op because no upload is performed when using the verifier
            #   but, it will allow us to update the egg!
            args = ["insights-client", "-m", "insights.client.apps.ansible.playbook_verifier",
                    "--quiet", "--payload", "noop", "--content-type", "noop"]
            env = {"PATH": BASIC_PATH}
            if config["insights_core_gpg_check"] == False:
                args.append("--no-gpg")
                env["BYPASS_GPG"] = "True"
            verifyProc = Popen(
                args,
                stdin=PIPE, stdout=PIPE, stderr=PIPE,
                env=env)
            stdout, stderr = verifyProc.communicate(input=playbook_str)
            if verifyProc.returncode != 0:
                _log("ERROR: Unable to verify playbook:\n%s\n%s" %
                (stdout.decode("utf-8"), stderr.decode("utf-8")))
                raise Exception
            verified = stdout.decode("utf-8")
            _log("Playbook verified.")
        else:
            _log("WARNING: Playbook verification disabled.")
            verified = playbook_str.decode("utf-8")
        try:
            playbook = yaml.safe_load(verified)
        except yaml.composer.ComposerError as e:
            _log("ERROR: Could not parse playbook")
            _log(str(e))
            raise

        for item in playbook:
            # remove signature field, ansible-runner dislikes bytes
            try:
                item.get('vars', {}).pop('insights_signature', None)
            except AttributeError:
                # vars was null/empty
                pass

        _log("Starting playbook run...")
        # required event for cloud connector
        on_start = executor_on_start(correlation_id=crc_dispatcher_correlation_id)
        events.addEvent(on_start)

        # run playbook
        runnerThread, runner = ansible_runner.interface.run_async(
            playbook=playbook,
            envvars={"PYTHONPATH": WORKER_LIB_DIR, "PATH": BASIC_PATH},
            event_handler=events.addEvent,
            quiet=True)

        # initialize elapsed counter
        elapsedTime = 0
        startTime = time.time()
        while runnerThread.is_alive():
            elapsedTime = time.time() - startTime
            if elapsedTime >= response_interval:
                # hit the interval, post events
                _log("Hit the response interval. Posting current status...")
                returnedEvents = _composeDispatcherMessage(events, return_url, response_to)
                response = self.dispatcher.Send(returnedEvents)
                # reset interval timer
                elapsedTime = 0
                startTime = time.time()

        if runner.status == 'failed':
            # last event sould be the failure, find the reason
            errorCode, errorDetails = _parseFailure(events[-1])
            if errorCode == "ANSIBLE_PLAYBOOK_NOT_INSTALLED":
                _log("WARNING: The rhc-worker-playbook package requires the ansible package to be installed.")
            # required event for cloud connector
            on_failed = executor_on_failed(correlation_id=crc_dispatcher_correlation_id, error_code=errorCode, error_details=errorDetails)
            events.addEvent(on_failed)

        # send the final message after playbook completed
        _log("Playbook run complete.")
        _log(str(events))
        returnedEvents = _composeDispatcherMessage(events, return_url, response_to)
        _log("Posting events...")
        response = self.dispatcher.Send(returnedEvents)
        _log("Post complete.")
        return

def serve():
    # load config to get directive
    config = _loadConfig()

    # open the channel to ygg Dispatcher
    channel = grpc.insecure_channel(YGG_SOCKET_ADDR)
    dispatcher = yggdrasil_pb2_grpc.DispatcherStub(channel)
    _log("Registering with directive %s..." % config["directive"])
    registrationResponse = dispatcher.Register(
        yggdrasil_pb2.RegistrationRequest(
            handler=config["directive"],
            detached_content=True,
            pid=os.getpid()))
    registered = registrationResponse.registered
    if not registered:
        _log("ERROR: Could not register rhc-worker-playbook.")
        sys.exit(1)
    _log("Registered rhc-worker-playbook.")
    address = registrationResponse.address.replace("@", "unix-abstract:")

    # create server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    try:
        yggdrasil_pb2_grpc.add_WorkerServicer_to_server(WorkerService(dispatcher=dispatcher), server)
    except ValueError as e:
        _log(str(e))
        raise
    server.add_insecure_port(address)

    # off to the races
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()
