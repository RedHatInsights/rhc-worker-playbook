from __future__ import print_function
import logging
import grpc
import os
from protocol import yggdrasil_pb2
from protocol import yggdrasil_pb2_grpc

with open(os.path.abspath("sample_playbook.yml")) as p:
    sample_playbook = p.read()

def run():
    with grpc.insecure_channel('localhost:50051') as channel:
        stub = yggdrasil_pb2_grpc.WorkerStub(channel)
        response = stub.Send(yggdrasil_pb2.Data(payload=sample_playbook.encode('utf-8')))
        # TODO: is this true/false or should there be try/except
        if response:
            print("Worker received message.")
        else:
            print("Something went wrong.")

if __name__ == '__main__':
    logging.basicConfig()
    run()
