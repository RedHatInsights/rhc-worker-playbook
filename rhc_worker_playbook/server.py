import sys
import os
from .constants import WORKER_LIB_DIR
sys.path.append(WORKER_LIB_DIR)
import yaml
import grpc
import ansible_runner
import time
import json
from concurrent import futures
from .protocol import yggdrasil_pb2_grpc, yggdrasil_pb2
from .dispatcher_events import executor_on_start, executor_on_failed
# from insights.client.core.apps.ansible.playbook_verifier import verify

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
        playbook_str = request.payload
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
        on_failed = executor_on_failed()

        for evt in runner.events:
            # print(evt)
            events.append(evt)

        returnedEvents = yggdrasil_pb2.Data(payload=json.dumps(events).encode('utf-8'))
        response = self.dispatcher.Send(returnedEvents)
        return yggdrasil_pb2.Receipt()

def serve():
    # open the channel to ygg Dispatcher
    channel = grpc.insecure_channel(YGG_SOCKET_ADDR)
    dispatcher = yggdrasil_pb2_grpc.DispatcherStub(channel)
    registrationResponse = dispatcher.Register(
        yggdrasil_pb2.RegistrationRequest(
            handler="rhc-worker-playbook",
            detached_payload=True,
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

