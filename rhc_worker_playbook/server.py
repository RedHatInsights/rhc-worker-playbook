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
# from insights.client.core.apps.ansible.playbook_verifier import verify

INSIGHTS_INGRESS_URL = os.environ.get('INSIGHTS_INGRESS_URL') or "https://cert.cloud.redhat.com/api/ingress/v1/upload"
YGG_SOCKET_ADDR = os.environ.get('YGG_SOCKET_ADDR')
if not YGG_SOCKET_ADDR:
    print("Missing YGG_SOCKET_ADDR environment variable")
    sys.exit(1)
# massage the value for python grpc
YGG_SOCKET_ADDR = YGG_SOCKET_ADDR.replace("unix:@", "unix-abstract:")

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
        events = []
        # parse playbook from data field
        playbook_str = request.content
        response_to = request.message_id
        # message body has:
        #   playbook string
        #   interval with which to send status
        #   crc replication ID

        # TODO: call insights-core lib to verify playbook
        # playbook = verify(playbook)
        playbook = yaml.safe_load(playbook_str.decode('utf-8'))
        # run playbook
        runner = ansible_runner.interface.run(playbook=playbook)

        # required event for cloud connector
        on_start = executor_on_start()
        events.append(on_start)

        # required event for cloud connector
        # TODO: send this if the job fails to start
        on_failed = executor_on_failed()

        for evt in runner.events:
            events.append(evt)

        # generate the request body to ingress (form data)
        # TODO?: generate by hand so request isn't a dependency
        # TODO: change the content type to what it should be
        req = Request('POST', INSIGHTS_INGRESS_URL, files={
                           "file": ("runner-events", json.dumps(events), "application/vnd.redhat.test.collection+tgz"),
                           "metadata": {}
                       }).prepare()

        returnedEvents = yggdrasil_pb2.Data(
            message_id=str(uuid.uuid4()).encode('utf-8'),
            content=req.body,
            directive=INSIGHTS_INGRESS_URL,
            metadata=req.headers,
            response_to=response_to)
        response = self.dispatcher.Send(returnedEvents)
        return yggdrasil_pb2.Receipt()

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

