import grpc
import time
from concurrent import futures
from protocol import yggdrasil_pb2_grpc, yggdrasil_pb2


class WorkerService(yggdrasil_pb2_grpc.WorkerServicer):

    def __init__(self, *args, **kwargs):
        pass

    def Start(self, request, context):

        # get the string from the incoming request
        message = request.message
        result = f'Hello I am up and running received "{message}" message from you'
        result = {'message': result, 'received': True}

        return pb2.MessageResponse(**result)

    def Status(self, request, context):

        # get the string from the incoming request
        message = request.message
        result = f'Hello I am up and running received "{message}" message from you'
        result = {'message': result, 'received': True}

        return pb2.MessageResponse(**result)

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    pb2_grpc.add_WorkerServicer_to_server(WorkerService(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    serve()
