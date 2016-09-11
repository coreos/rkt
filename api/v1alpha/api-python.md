# API RKT Python

This is a example of api client in the python.

How you can use this client? Simply

1) Install grpcio-tools over pip this is module for Python
   ```
   pip install grpcio-tools
   ```
2) Create api_pb2.py from api.proto by this command.
   ```
   python -m grpc.tools.protoc -I. --python_out=. --grpc_python_out=. api.proto
   ```
That is all, now you can use the client
