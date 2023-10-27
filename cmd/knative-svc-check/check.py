import os
import requests
from kubernetes import client, config
from time import sleep
from kh_client import *

kh_url   = os.environ["KH_REPORTING_URL"]
kh_att   = int(os.environ["KH_ATT"])
kh_del   = int(os.environ["KH_DEL"])
ks_image = os.environ["KS_IMAGE"]
ks_svc   = os.environ["KS_SVC"]
ks_ns    = os.environ["KS_NS"]

def create_knative_service(namespace, service_name, image):
    config.load_incluster_config()
    api_instance = client.CustomObjectsApi()

    # Clean up if already existing
    existing_service = None
    try:
        existing_service = api_instance.get_namespaced_custom_object(
            group="serving.knative.dev",
            version="v1",
            namespace=namespace,
            plural="services",
            name=service_name,
        )
    except Exception as e:
        pass

    if existing_service:
        delete_knative_service(namespace, service_name)
    # End of clean up

    service_url = f"http://{service_name}.{namespace}.svc.cluster.local"

    yaml_service = {
        "apiVersion": "serving.knative.dev/v1",
        "kind": "Service",
        "metadata": {"name": service_name},
        "spec": {
            "template": {
                "spec": {
                    "containers": [
                        {
                            "image": image,
                        }
                    ]
                }
            }
        }
    }

    api_instance.create_namespaced_custom_object(
        group="serving.knative.dev",
        version="v1",
        namespace=namespace,
        plural="services",
        body=yaml_service,
    )

    print("Service Knative successfully created")

def test_knative_service(namespace, service_name, max_attempts, wait_delay):
    sleep(wait_delay)
    service_url = f"http://{service_name}.{namespace}.svc.cluster.local"

    for attempt in range(max_attempts):
        try:
            response = requests.get(service_url)
            response.raise_for_status()
            print(f"Attempt #{attempt + 1} {response.status_code} : {response.text}")
            try:
                report_success()
            except Exception as e:
                print(f"Error when reporting success: {e}")
                exit(1)
            break
        except requests.exceptions.RequestException as e:
            print(f"Attempt #{attempt + 1} Error : {e}")
            if attempt < max_attempts - 1:
                print(f"New attempt in {wait_delay}s...")
                sleep(wait_delay)
    else:
        try:
            report_failure(["Can't reach the service"])
        except Exception as e:
              print(f"Error when reporting failure: {e}")
              exit(1)

def delete_knative_service(namespace, service_name):
    config.load_incluster_config()
    api_instance = client.CustomObjectsApi()

    try:
        api_instance.delete_namespaced_custom_object(
            group="serving.knative.dev",
            version="v1",
            namespace=namespace,
            plural="services",
            name=service_name,
        )
        print("Service Knative deleted")
    except Exception as e:
        print("Error while deleting : {e}")


if __name__ == "__main__":
    create_knative_service(ks_ns, ks_svc, ks_image)
    test_knative_service(ks_ns, ks_svc, kh_att, kh_del)
    delete_knative_service(ks_ns, ks_svc)
