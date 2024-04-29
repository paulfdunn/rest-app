#!/usr/bin/python3
# This script shows how to call the telemetry API from Python
import argparse
import json
import shutil
import ssl
import time
import urllib
import urllib.request

parser = argparse.ArgumentParser(
    prog='insplore.py',
    description='This script will make telemetry calls to a specified IP')

parser.add_argument('--ip', required=True,
                    help='IP of the telemetry target.')

args = parser.parse_args()

# The services are currently using a self-signed cert, and the following is used to
# make the urlopen not error.
sscContext = ssl.create_default_context()
sscContext.check_hostname = False
sscContext.verify_mode = ssl.CERT_NONE

# Get an auth token.
loginURL = f"https://{args.ip}:8000/auth/login/"
req = urllib.request.Request(
    loginURL, data=b'{"Email":"admin", "Password":"P@ss!234"}', method='PUT')
try:
    response = urllib.request.urlopen(req, context=sscContext)
except Exception as error:
    print(f"\nlogin error:{error}")
    exit()
token = response.read()
response.close()

# Create a task
taskURL = f"https://{args.ip}:8001/task/"
payload = {
    "Command": ["ls -al"],
}
headers = {
    "Authorization": "Bearer " + token.decode("utf-8")
}
req = urllib.request.Request(
    taskURL, data=json.dumps(payload).encode('utf-8'), headers=headers, method='POST')
try:
    response = urllib.request.urlopen(req, context=sscContext)
except Exception as error:
    print(f"\ntask create error:{error}")
    exit()
createdTask = json.loads(response.read())
response.close()
print(f"\ncreatedTask:{createdTask}")

# Get the task status and loop until completed.
completed = False
while not completed:
    statusURL = f"https://{args.ip}:8001/status/?uuid={createdTask['UUID']}"
    headers = {
        "Authorization": "Bearer " + token.decode("utf-8")
    }
    req = urllib.request.Request(
        statusURL, headers=headers, method='GET')
    try:
        response = urllib.request.urlopen(req, context=sscContext)
    except Exception as error:
        print(f"\nstatus get error:{error}")
        exit()
    statusGet = json.loads(response.read())
    response.close()
    # print(f"\nstatusGet:{statusGet}")
    if statusGet[0]['StatusString'] == 'Completed':
        completed = True
        break
    print(f"\nstatus received: " +
          f"{statusGet[0]['StatusString']}, waiting for status 'Completed'")
    time.sleep(5)
print(f"status at completion: {statusGet[0]}")

# Download the file
downloadURL = f"https://{args.ip}:8001/task/?uuid={createdTask['UUID']}"
headers = {
    "Authorization": "Bearer " + token.decode("utf-8")
}
req = urllib.request.Request(
    downloadURL, headers=headers, method='GET')
try:
    response = urllib.request.urlopen(req, context=sscContext)
except Exception as error:
    print(f"\nstatus get error:{error}")
    exit()
downloadFilename = "./telemetry.zip"
zip = open(downloadFilename, 'wb')
shutil.copyfileobj(response, zip)
response.close()
print(f"\ndownloaded file {downloadFilename}")
