package main

import (
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

var templ = template.Must(template.New("block").Parse(templateStr))

type CachedSite struct {
	response     *http.Response
	responseBody []byte
}

var blockedSet = map[string]bool{}
var cachedSites = map[string]CachedSite{}

func main() {
	server := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handler),
	}

	go func() {
		log.Fatal(http.ListenAndServe(":8081", http.HandlerFunc(consoleHandler)))
	}()
	log.Fatal(server.ListenAndServe())
}

func consoleHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received block request from %s\n", r.URL.String())
	url, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.HasPrefix(string(url), "block") {
		blockedSet[strings.Trim(string(url), "block ")] = true
		log.Printf("Blocked site: %s\n", url)
	} else if strings.HasPrefix(string(url), "unblock all") {
		blockedSet = map[string]bool{}
		log.Printf("Unblocked all blocked sites\n")
	} else if strings.HasPrefix(string(url), "unblock") {
		blockedSet[strings.Trim(string(url), "unblock ")] = false
		log.Printf("Unblocked site: %s\n", url)
	} else {
		log.Printf("Invalid command: %s\n", url)
		http.Error(w, "Invalid command", http.StatusBadRequest)
		return
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	for blockedURL, isBlocked := range blockedSet {
		if strings.Contains(r.URL.String(), blockedURL) && isBlocked {
			w.WriteHeader(http.StatusLocked)
			log.Printf("Request from site %s blocked\n.", r.URL.String())
			templ.Execute(w, r.URL.Host)
			return
		}
	}
	if r.Method == http.MethodConnect {
		handleHTTPS(w, r)
	} else {
		handleHTTP(w, r)
	}
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received HTTP request from %s\n", r.URL.String())

	// Check if the site is cached
	cachedSite, ok := cachedSites[r.URL.String()]
	if ok && isFresh(cachedSite.response) {
		log.Printf("Serving cached response for %s\n", r.URL.String())
		for head, values := range cachedSite.response.Header {
			for _, value := range values {
				w.Header().Add(head, value)
			}
		}
		w.Write(cachedSite.responseBody)
		log.Printf("Finished serving cached response\n")
		return
	}

	client := &http.Client{}
	proxyReq, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		log.Printf("Error creating request: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proxyReq.Header = r.Header
	res, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding to client: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Successfully forwarded the request\n")
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading response body: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if isCacheable(res) {
		cachedSites[r.URL.String()] = CachedSite{
			response:     res,
			responseBody: body,
		}
		log.Printf("Response cached\n")
	} else {
		log.Printf("Response not cacheable\n")
	}

	for head, values := range res.Header {
		for _, value := range values {
			log.Printf("Header[%q] = %q\n", head, value)
			w.Header().Add(head, value)
		}
	}
	w.Write(body)
	log.Printf("Finished cloning request\n")
}

func isFresh(response *http.Response) bool {
	if response == nil {
		return false
	}
	expires := response.Header.Get("Expires")
	if expires != "" {
		expiryTime, err := time.Parse(time.RFC1123, expires)
		if err != nil {
			return false
		}
		if expiryTime.Before(time.Now()) {
			return false
		}
	}

	return true
}

func isCacheable(response *http.Response) bool {
	cacheControl := response.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "private") {
		return false
	}
	return true
}

func handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("Reccieved HTTPS request from %s/n ", r.URL.String())
	dest_conn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	// Hijack the connection. This means we are now responsible
	// for manually closing the TCP connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	// Start two way connection between the client and server
	// Communication from server to client on one goroutine
	// Communication from client to server on another goroutine
	go transfer(dest_conn, client_conn)
	go transfer(client_conn, dest_conn)
}
func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

const templateStr = `
<html>
<head>
<title>Blocked Site</title>
</head>
<body>
<br> {{ . }} has been blocked. Type 'unblock {{ . }}' on proxy management console to view.
</body>
</html>
`
