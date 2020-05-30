package main

import (
	_ "github.com/mattn/go-sqlite3"
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	// "github.com/buger/jsonparser"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{}

var (
	all *bool
	id *string
	db *sql.DB
	dsn DSN
	SENTRY_URL string 
	exists bool
	projects map[string]*DSN
)

type DSN struct { 
	rawurl string
	key string
	projectId string
}
func (d DSN) storeEndpoint() string {
	// return strings.Join([]string{"http://sentry.io/api/",d.projectId,"/store/?sentry_key=",d.key,"&sentry_version=7"}, "")
	return strings.Join([]string{"http://localhost:9000/api/",d.projectId,"/store/?sentry_key=",d.key,"&sentry_version=7"}, "")
}
func newDSN(rawurl string) (*DSN) {
	// TODO if 'sentry.io' in url then host := sentry.io else host := localhost:9000, and update storeEndpoint() w/ 'host'
	key := strings.Split(rawurl, "@")[0][7:]

	uri, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	idx := strings.LastIndex(uri.Path, "/")
	if idx == -1 {
		log.Fatal("missing projectId in dsn")
	}
	projectId := uri.Path[idx+1:]
	fmt.Println("> PROJECTID", projectId)
	
	return &DSN{
		rawurl,
		key,
		projectId,
	}
}

type Event struct {
	id int
	name, _type string
	headers []byte
	bodyBytes []byte
}
func (e Event) String() string {
	return fmt.Sprintf("> event id, type: %v %v", e.id, e._type)
}

func init() {
	defer fmt.Println("> init() complete")
	
	if err := godotenv.Load(); err != nil {
        log.Print("No .env file found")
	}

	projects = make(map[string]*DSN)
	projects["javascript"] = newDSN(os.Getenv("DSN_REACT"))
	projects["python"] = newDSN(os.Getenv("DSN_PYTHON"))
	// projects["javascript"] = newDSN(os.Getenv("DSN_REACT_SAAS"))
	// projects["python"] = newDSN(os.Getenv("DSN_PYTHON_SAAS"))

	all = flag.Bool("all", false, "send all events or 1 event from database")
	id = flag.String("id", "", "id of event in sqlite database")
	flag.Parse()
	fmt.Printf("> --all= %v\n", *all)
	fmt.Printf("> --id= %v\n", *id)

	
	db, _ = sql.Open("sqlite3", "sqlite.db")
}

func javascript(bodyBytes []byte, headers []byte) {
	fmt.Println("> javascript")
	
	bodyInterface := unmarshalJSON(bodyBytes)
	bodyInterface = replaceEventId(bodyInterface)
	bodyInterface = addTimestamp(bodyInterface)
	
	bodyBytesPost := marshalJSON(bodyInterface)
	
	SENTRY_URL = projects["javascript"].storeEndpoint()
	request, errNewRequest := http.NewRequest("POST", SENTRY_URL, bytes.NewReader(bodyBytesPost))
	if errNewRequest != nil { log.Fatalln(errNewRequest) }
	
	headerInterface := unmarshalJSON(headers)
	
	for _, v := range [4]string{"Accept-Encoding","Content-Length","Content-Type","User-Agent"} {
		request.Header.Set(v, headerInterface[v].(string))
	}
	
	response, requestErr := httpClient.Do(request)
	if requestErr != nil { fmt.Println(requestErr) }

	responseData, responseDataErr := ioutil.ReadAll(response.Body)
	if responseDataErr != nil { log.Fatal(responseDataErr) }

	fmt.Printf("> javascript event response: %v\n", string(responseData))
}

func python(bodyBytesCompressed []byte, headers []byte) {
	fmt.Println("> python")
	
	bodyBytes := decodeGzip(bodyBytesCompressed)
	bodyInterface := unmarshalJSON(bodyBytes)
	
	bodyInterface = replaceEventId(bodyInterface)
	bodyInterface = replaceTimestamp(bodyInterface)
	
	bodyBytesPost := marshalJSON(bodyInterface)
	buf := encodeGzip(bodyBytesPost)
	
	SENTRY_URL = projects["python"].storeEndpoint()
	request, errNewRequest := http.NewRequest("POST", SENTRY_URL, &buf)
	if errNewRequest != nil { log.Fatalln(errNewRequest) }

	headerInterface := unmarshalJSON(headers)

	for _, v := range [5]string{"Accept-Encoding","Content-Length","Content-Encoding","Content-Type","User-Agent"} {
		request.Header.Set(v, headerInterface[v].(string))
	}

	response, requestErr := httpClient.Do(request)
	if requestErr != nil { fmt.Println(requestErr) }

	responseData, responseDataErr := ioutil.ReadAll(response.Body)
	if responseDataErr != nil { log.Fatal(responseDataErr) }

	fmt.Printf("> python event response: %v\n", string(responseData))
}

func main() {
	defer db.Close()
	

	rows, err := db.Query(strings.ReplaceAll("SELECT * FROM events WHERE id=?", "?", *id))

	// ORIGINAL
	// rows, err := db.Query("SELECT * FROM events ORDER BY id DESC")
	
	if err != nil {
		fmt.Println("Failed to load rows", err)
	}
	for rows.Next() {
		var event Event
		rows.Scan(&event.id, &event.name, &event._type, &event.bodyBytes, &event.headers)
		fmt.Println(event)

		if (event._type == "javascript") {
			javascript(event.bodyBytes, event.headers)
		}

		if (event._type == "python") {
			python(event.bodyBytes, event.headers)
		}

		if !*all {
			rows.Close()
		}
	}
	rows.Close()
}

func decodeGzip(bodyBytesInput []byte) (bodyBytesOutput []byte) {
	bodyReader, err := gzip.NewReader(bytes.NewReader(bodyBytesInput))
	if err != nil {
		fmt.Println(err)
	}
	bodyBytesOutput, err = ioutil.ReadAll(bodyReader)
	if err != nil {
		fmt.Println(err)
	}
	return
}

func encodeGzip(b []byte) bytes.Buffer {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(b)
	w.Close()
	// return buf.Bytes()
	return buf
}

func unmarshalJSON(bytes []byte) map[string]interface{} {
	var _interface map[string]interface{}
	if err := json.Unmarshal(bytes, &_interface); err != nil {
		panic(err)
	}
	return _interface
}

func marshalJSON(bodyInterface map[string]interface{}) []byte {
	bodyBytes, errBodyBytes := json.Marshal(bodyInterface) 
	if errBodyBytes != nil { fmt.Println(errBodyBytes)}
	return bodyBytes
}

func replaceEventId(bodyInterface map[string]interface{}) map[string]interface{} {
	if _, ok := bodyInterface["event_id"]; !ok { 
		log.Print("no event_id on object from DB")
	}

	fmt.Println("> before",bodyInterface["event_id"])
	var uuid4 = strings.ReplaceAll(uuid.New().String(), "-", "") 
	bodyInterface["event_id"] = uuid4
	fmt.Println("> after ",bodyInterface["event_id"])
	return bodyInterface
}

func replaceTimestamp(bodyInterface map[string]interface{}) map[string]interface{} {
	fmt.Println("before",bodyInterface["timestamp"])
	timestamp := time.Now()
	oldTimestamp := bodyInterface["timestamp"].(string)
	newTimestamp := timestamp.Format("2006-01-02") + "T" + timestamp.Format("15:04:05")
	bodyInterface["timestamp"] = newTimestamp + oldTimestamp[19:]
	fmt.Println("after ",bodyInterface["timestamp"])
	return bodyInterface
}
// SDK's are supposed to set timestamps https://github.com/getsentry/sentry-javascript/issues/2573
// Newer js sdk provides timestamp, so stop calling this function, upon upgrading js sdk. 
func addTimestamp(bodyInterface map[string]interface{}) map[string]interface{} {
	log.Print("no timestamp on object from DB")
	timestamp1 := time.Now()
	newTimestamp1 := timestamp1.Format("2006-01-02") + "T" + timestamp1.Format("15:04:05")
	bodyInterface["timestamp"] = newTimestamp1 + ".118356Z"
	fmt.Println("> after ",bodyInterface["timestamp"])
	return bodyInterface
}