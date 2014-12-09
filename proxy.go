package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Port int
	ProxyHost string
	ProxyUser string
	ProxyPass string
}

const CONFIG_FILE = "config.yml"
var config Config
var proxyAuthorization string

func handleHttps(w http.ResponseWriter, r *http.Request) {
	hj, _ := w.(http.Hijacker)
	if proxyConn, err := net.Dial("tcp", config.ProxyHost); err != nil {
		log.Fatal(err)
	} else if clientConn, _, err := hj.Hijack(); err != nil {
		proxyConn.Close()
		log.Fatal(err)
	} else {
		r.Header.Set("Proxy-Authorization", proxyAuthorization)
		r.Write(proxyConn)
		go func() {
			io.Copy(clientConn, proxyConn)
			proxyConn.Close()
		}()
		go func() {
			io.Copy(proxyConn, clientConn)
			clientConn.Close()
		}()
	}
}

func handleHttp(w http.ResponseWriter, r *http.Request) {
	hj, _ := w.(http.Hijacker)
	client := &http.Client{}
	r.RequestURI = ""
	if resp, err := client.Do(r); err != nil {
		log.Fatal(err)
	} else if conn, _, err := hj.Hijack(); err != nil {
		log.Fatal(err)
	} else {
		defer conn.Close()
		resp.Write(conn)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[%s] %s", r.Method, r.URL)
	if r.Method == "CONNECT" {
		handleHttps(w, r)
	} else {
		handleHttp(w, r)
	}
}

func loadConfig() {
	contents, err := ioutil.ReadFile(CONFIG_FILE)
	if err != nil {
		fmt.Println(CONFIG_FILE, err)
		os.Exit(1)
	}
	yaml.Unmarshal(contents, &config)
	proxyAuthorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(config.ProxyUser+":"+config.ProxyPass))
	fmt.Printf("%+v\n", config)
}

func main() {
	loadConfig()
	proxyUrlString := fmt.Sprintf("http://%s:%s@%s", url.QueryEscape(config.ProxyUser), url.QueryEscape(config.ProxyPass), config.ProxyHost)
	proxyUrl, err := url.Parse(proxyUrlString)
	if err != nil {
		log.Fatal(err)
	}
	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}

	handler := http.HandlerFunc(handleRequest)
	listen := fmt.Sprintf("localhost:%d", config.Port)
	log.Printf("Start serving on %s", listen)
	log.Fatal(http.ListenAndServe(listen, handler))
}
