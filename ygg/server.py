import grpc
import time
from concurrent import futures
from .protocol import yggdrasil_pb2_grpc, yggdrasil_pb2

import sys
import os

sys.path.append(os.path.join(os.path.dirname(__file__), 'contrib'))
import ansible_runner
sys.path.pop()

class WorkerService(yggdrasil_pb2_grpc.WorkerServicer):

    def __init__(self, *args, **kwargs):
        pass

    def Start(self, request, context):
        # parse playbook from data field
        playbook_str = request.data
        # run sample playbook
        runner = ansible_runner.interface.run(playbook=playbook_str.decode('utf-8'))
        print(runner)
        if True:
            accepted = True
        else:
            accepted = False

        return yggdrasil_pb2.StartResponse(accepted=True)

    def Status(self, request, context):

        # get the string from the incoming request
        message = request.message
        result = f'Hello I am up and running received "{message}" message from you'
        result = {'message': result, 'received': True}

        return pb2.MessageResponse(**result)

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    yggdrasil_pb2_grpc.add_WorkerServicer_to_server(WorkerService(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()
