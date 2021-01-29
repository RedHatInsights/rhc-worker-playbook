import sys
import os
from .constants import WORKER_LIB_DIR
sys.path.append(WORKER_LIB_DIR)
import yaml
import grpc
import ansible_runner
import time
from concurrent import futures
from .protocol import yggdrasil_pb2_grpc, yggdrasil_pb2

YGG_SOCKET_ADDR = os.environ.get('YGG_SOCKET_ADDR')
if not YGG_SOCKET_ADDR:
    print("Missing YGG_SOCKET_ADDR environment variable")
    sys.exit(1)
# massage the value for python grpc
YGG_SOCKET_ADDR = YGG_SOCKET_ADDR.replace("unix:@", "unix-abstract:")

class WorkerService(yggdrasil_pb2_grpc.WorkerServicer):

    def __init__(self, *args, **kwargs):
        pass

    def Send(self, request, context):
        # parse playbook from data field
        playbook_str = request.payload
        # TODO: call insights-core lib to verify playbook
        playbook = yaml.safe_load(playbook_str.decode('utf-8'))
        # run playbook
        runner = ansible_runner.interface.run(playbook=playbook)

        return yggdrasil_pb2.Receipt()

def serve():
    with grpc.insecure_channel(YGG_SOCKET_ADDR) as channel:
        # register with yggd dispatcher
        dispatcherClient = yggdrasil_pb2_grpc.DispatcherStub(channel)
        registrationResponse = dispatcherClient.Register(yggdrasil_pb2.RegistrationRequest(handler="rhc-ansible-worker", pid=os.getpid()))
        registered = registrationResponse.registered
        if not registered:
            print("Could not register rhc-ansible-worker.")
            sys.exit(1)
        address = registrationResponse.address.replace("@", "unix-abstract:")

    # create server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    yggdrasil_pb2_grpc.add_WorkerServicer_to_server(WorkerService(), server)
    server.add_insecure_port(address)
    # off to the races
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()

