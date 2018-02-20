package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

type HeaderReporter interface {
	//SubmitHeader reports a solved header
	Submit(header []byte) (err error)
}

// Client is used to connect to siad
type Client struct {
	pool string
	account string
    miners []*Miner
    conn net.Conn
}

type jhdr struct {
    Id int32 `json:"id"`
    Jsonrpc string `json:"jsonrpc"`
}
type jbody struct {
    Method string `json:"method"`
    Params []string `json:"params"`
}
type jmsg struct{
    jhdr
    jbody
}

func (pool *Client) Monitor() (error) {
    var err error
    conn, err := net.Dial("tcp", pool)
    defer conn.Close()
    if err!= nil { return err)

    params := []string{account}
    msg := jmsg{1, "2.0", "eth_submitLogin", params}
    data, _ := json.Marshal(msg)
    conn.Write(data)

    const bufSize = 2048
    buf := make([]byte, bufSize)
    response := jhdr{}
    // handle incoming json messages
    for true { 
        buf = buf[:bufSize]
        n, _ := conn.Read(buf)
        buf = buf[:n]
        jerr = json.Unmarshal(buf, &response)
        if jerr != nil { fmt.Println(jerr) }

        if response.Id != 0 { continue }
        var rcvd struct{Result []string `json:"result"`}
        jerr = json.Unmarshal(buf, &rcvd)
        if jerr != nil { fmt.Println(jerr) }
    }
}

func decodeMessage(resp *http.Response) (msg string, err error) {
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var data struct{Message string `json:"message"`}
	if err = json.Unmarshal(buf, &data); err == nil {
		msg = data.Message
	}
	return
}

//GetHeaderForWork fetches new work from the SIA daemon
func (sc *Client) GetHeaderForWork(longpoll bool) (target, header []byte, err error) {
	timeout := time.Second * 10
	if longpoll {
		timeout = time.Minute * 60
	}
	client := &http.Client{
		Timeout: timeout,
	}

	url := sc.siadurl
	if longpoll {
		url += "&longpoll"
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
	case 400:
		msg, errd := decodeMessage(resp)
		if errd != nil {
			err = fmt.Errorf("status code %d, no message", resp.StatusCode)
		} else {
			err = fmt.Errorf("status code %d, message: %s", resp.StatusCode, msg)
		}
		return
	default:
		err = fmt.Errorf("status code %d", resp.StatusCode)
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if len(buf) < 112 {
		err = fmt.Errorf("Invalid response, only received %d bytes", len(buf))
		return
	}

	target = buf[:32]
	header = buf[32:112]

	xme := resp.Header.Get("X-Mining-Extensions")
	if strings.Contains(xme, "longpoll") {
	}

	return
}

//SubmitHeader reports a solved header to the SIA daemon
func (sc *Client) SubmitHeader(header []byte) (err error) {
	req, err := http.NewRequest("POST", sc.siadurl, bytes.NewReader(header))
	if err != nil {
		return
	}

	req.Header.Add("User-Agent", "Sia-Agent")

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	switch resp.StatusCode {
	case 204:
	default:
		msg, errd := decodeMessage(resp)
		if errd != nil {
			err = fmt.Errorf("status code %d", resp.StatusCode)
		} else {
			err = fmt.Errorf("%s", msg)
		}
		return
	}
	return
}
