import sys
from .constants import WORKER_LIB_DIR
sys.path.append(WORKER_LIB_DIR)
import yaml
import grpc
import ansible_runner
import time
from concurrent import futures
from .protocol import yggdrasil_pb2_grpc, yggdrasil_pb2

class WorkerService(yggdrasil_pb2_grpc.WorkerServicer):

    def __init__(self, *args, **kwargs):
        pass

    def Send(self, request, context):
        # parse playbook from data field
        playbook_str = request.payload
        playbook = yaml.safe_load(playbook_str.decode('utf-8'))
        # run sample playbook
        runner = ansible_runner.interface.run(playbook=playbook)

        return yggdrasil_pb2.Receipt()

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    yggdrasil_pb2_grpc.add_WorkerServicer_to_server(WorkerService(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()
