#!/usr/bin/python
# -*- coding: utf-8 -*-
"""
Copyright 2016 Author Josef Boudnik - josef.boudnik@seznam.cz

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
istributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
"""

from grpc.beta import implementations
import api_pb2

host = "localhost"
port = 15441
timeout = 3


def run():
        channel = implementations.insecure_channel(host, port)
        connect = api_pb2.beta_create_PublicAPI_stub(channel)
        del channel

        list_pods = connect.ListPods(api_pb2.ListPodsRequest(), timeout)
        pods_running = filter(lambda pod: pod.state is api_pb2.POD_STATE_RUNNING, list_pods.pods)
        for pod_running in pods_running:
            print 'Pod "%s" is running' % pod_running.id

        list_images = connect.ListImages(api_pb2.ListImagesRequest(), timeout)
        images = filter(lambda image: "coreos.com" in image.name, list_images.images)
        for list_prefix in images:
            print 'Found image "%s"' % list_prefix.name

if __name__ == '__main__':
    run()
