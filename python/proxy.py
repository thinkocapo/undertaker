import os
import datetime
from dotenv import load_dotenv
from flask import Flask, request, json, abort
from flask_cors import CORS
import json
# import sentry_sdk
# from sentry_sdk.integrations.flask import FlaskIntegration
from services import compress_gzip, decompress_gzip
import sqlite3
import string # ?
import urllib3
import uuid
load_dotenv()
http = urllib3.PoolManager()

app = Flask(__name__)
# app.run(ssl_context='adhoc') # flask run --cert=adhoc
CORS(app)

print("""
                               Welcome To The
  _   _   _   _   ____    _____   ____    _____      _      _  __  _____   ____  
 | | | | | \ | | |  _ \  | ____| |  _ \  |_   _|    / \    | |/ / | ____| |  _ \ 
 | | | | |  \| | | | | | |  _|   | |_) |   | |     / _ \   | ' /  |  _|   | |_) |
 | |_| | | |\  | | |_| | | |___  |  _ <    | |    / ___ \  | . \  | |___  |  _ < 
  \___/  |_| \_| |____/  |_____| |_| \_\   |_|   /_/   \_\ |_|\_\ |_____| |_| \_\
                                                                                 
""")

SENTRY=''

def sentryUrl(DSN):
    print('\nsentryUrl')
    # DSN = os.getenv('DSN_PYTHON')
    KEY = DSN.split('@')[0][7:]
    PROJECT_ID= DSN[-1:]
    # Must pass auth key in URL (not request headers) or else 403 CSRF error from Sentry
    # SENTRY ="http://localhost:9000/api/{}/store/?sentry_key=09aa0d909232457a8a6dfff118bac658&sentry_version=7".format(PROJECT_ID)
    # TODO must update for http://sentry.io as well. or re-write proxy in Go 
    return "http://localhost:9000/api/%s/store/?sentry_key=%s&sentry_version=7" % (PROJECT_ID, KEY)

# DATABASE - Must be full absolute path to sqlite database file
# sqlite.db will get created if doesn't exist
SQLITE = os.getenv('SQLITE')
database = SQLITE or os.getcwd() + "/sqlite.db"
print("> database", database)
with sqlite3.connect(database) as db:
    # TODO platform, eventType instead of name, type. for now, re-purpose 'name' as 'platform' and 'type' as 'eventType'
    cursor = db.cursor()
    cursor.execute(""" CREATE TABLE IF NOT EXISTS events (
                                            id integer PRIMARY KEY,
                                            name text,
                                            type text,
                                            data BLOB,
                                            headers BLOB
                                        ); """)
    cursor.close()

########################  STEP 1  #########################

# MODIFIED_DSN_FORWARD - Intercepts the payload sent by sentry_sdk in event.py, and then sends it to a Sentry instance
@app.route('/api/2/store/', methods=['POST'])
def forward():
    print('> 0000000000 FORWARD')

    # print('> type(request.data)', type(request.data))
    # print('> type(request_headers)', type(request.headers))

    # print('\n111 REQUEST headers\n', request.headers)
    # print('\n111 REQUEST body\n', json.dumps(json.loads(decompress_gzip(request.data)),indent=2))

    # TODO https://github.com/thinkocapo/undertaker/issues/48
    def make(headers):
        request_headers = {}
        user_agent = request.headers.get('User-Agent')
        if 'ython' in user_agent:
            print('> python error')
            for key in ['Accept-Encoding','Content-Length','Content-Encoding','Content-Type','User-Agent']:
                request_headers[key] = request.headers.get(key)
                # TODO Original, 'flask' project
                SENTRY = sentryUrl(os.getenv('DSN_PYTHON'))
                # SENTRY = sentryUrl(os.getenv('DSN_PYTHON_SAAS'))
                # SENTRY = sentryUrl(os.getenv('DSN_PYTHONEAT_SAAS'))
        if 'ozilla' in user_agent or 'hrome' in user_agent or 'afari' in user_agent:
            print('> javascript error')
            for key in ['Accept-Encoding','Content-Length','Content-Type','User-Agent']:
                request_headers[key] = request.headers.get(key)
                SENTRY = sentryUrl(os.getenv('DSN_REACT'))
                # SENTRY = sentryUrl(os.getenv('DSN_REACT_SAAS'))
        return request_headers, SENTRY

    request_headers, SENTRY = make(request.headers)
    print('> SENTRY url is', SENTRY)
    # return 'END EXPERIMENT'

    try:
        print('> type(request.data)', type(request.data))
        print('> type(request_headers)', type(request_headers))

        # response = http.request(
        #     "POST", str(SENTRY), body=request.data, headers=request_headers 
        # )

        print('> nothing saved to sqlite database')
        return 'success'
    except Exception as err:
        # print('LOCAL EXCEPTION', err)
        print('DONE')

# MODIFIED_DSN_SAVE - Intercepts event from sentry sdk and saves them to Sqlite DB. No forward of event to your Sentry instance.
@app.route('/api/3/store/', methods=['POST'])
def save():
    print('> SAVING')

    event_type = ''
    request_headers = {}
    user_agent = request.headers.get('User-Agent')
    
    if 'ython' in user_agent:
        print('> python error type')
        event_type = 'python'
        for key in ['Accept-Encoding','Content-Length','Content-Encoding','Content-Type','User-Agent']:
            request_headers[key] = request.headers.get(key)
    if 'ozilla' in user_agent or 'hrome' in user_agent or 'afari' in user_agent:
        print('> javascript error type')
        event_type = 'javascript'
        for key in ['Accept-Encoding','Content-Length','Content-Type','User-Agent']:
            request_headers[key] = request.headers.get(key)

    # request.data for javascript going into sqlite should be fine...
    # request_headers = {}
    # for key in ['Host','Accept-Encoding','Content-Length','Content-Encoding','Content-Type','User-Agent']:
    #     request_headers[key] = request.headers.get(key)

    insert_query = ''' INSERT INTO events(name,type,data,headers)
              VALUES(?,?,?,?) '''
    record = (event_type, event_type, request.data, json.dumps(request_headers))
   
    try:
        with sqlite3.connect(database) as db:
            cursor = db.cursor()
            cursor.execute(insert_query, record)
            print('> Id in Sqlite', cursor.lastrowid)
            cursor.close()
            return str(cursor.lastrowid)
    except Exception as err:
        print("LOCAL EXCEPTION", err)

# MODIFIED_DSN_SAVE_AND_FORWARD
@app.route('/api/4/store/', methods=['POST'])
def save_and_forward():

    request_headers = {}
    for key in ['Host','Accept-Encoding','Content-Length','Content-Encoding','Content-Type','User-Agent']:
        request_headers[key] = request.headers.get(key)

    insert_query = ''' INSERT INTO events(name,type,data,headers)
              VALUES(?,?,?,?) '''
    record = ('python', 'example', request.data, json.dumps(request_headers)) # type(json.dumps(request_headers)) <type 'str'>

    try:
        with sqlite3.connect(database) as db:
            cursor = db.cursor()
            cursor.execute(insert_query, record)
            print('> sqlite3 ID', cursor.lastrowid)
            cursor.close()
    except Exception as err:
        print("LOCAL EXCEPTION SAVE", err)

    try:
        response = http.request(
            "POST", str(SENTRY), body=request.data, headers=request_headers 
        )
        return 'success'
    except Exception as err:
        print('LOCAL EXCEPTION FORWARD', err)

# @app.route('/load-and-forward', defaults={'_id':0}, methods=['GET'])
# @app.route('/load-and-forward/<_id>', methods=['GET'])
# def load_and_forward(_id):
