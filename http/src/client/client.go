package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

var ()

// https://www.rfc-editor.org/rfc/rfc8693.html#section-2.2.1
type TokenResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type,omitempty"`
	ExpiresIn       int64  `json:"expires_in,omitempty"`
	Scope           string `json:"scope,omitempty"`
	RefreshToken    string `json:"refresh_token,omitempty"`
}

func main() {

	resourceAddress := flag.String("resourceAddress", "https://server.domain.com:8443", "host:port of Resource Server")
	resourceSNI := flag.String("resourceSNI", "server.domain.com", "SNI of Resource Server")
	tlsCA := flag.String("tlsCA", "tls-ca.crt", "CACert for server")
	tlsCert := flag.String("tlsCert", "alice.crt", "TLS Client Certificate")
	tlsKey := flag.String("tlsKey", "alice.key", "TLS Client Key")
	stsSNI := flag.String("stsSNI", "sts.domain.com", "SNI of the STS Server")

	stsAddress := flag.String("stsaddress", "https://sts.domain.com:8081", "STS Server address")
	stsAudience := flag.String("stsaudience", "https://server.domain.com:8443", "the audience and resource value to send to STS server")
	stsScope := flag.String("scope", "https://www.googleapis.com/auth/cloud-platform", "scope to send to STS server")

	stsCred := flag.String("stsCred", "iamtheeggman", "STS Credentials inline")

	flag.Parse()
	caCert, err := ioutil.ReadFile(*tlsCA)
	if err != nil {
		log.Fatalf("did not read tlsCA: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	clientCerts, err := tls.LoadX509KeyPair(
		*tlsCert,
		*tlsKey,
	)
	if err != nil {
		log.Fatalf("did not read client certs: %v", err)
	}

	stsTLSConfig := &tls.Config{
		ServerName:   *stsSNI,
		Certificates: []tls.Certificate{clientCerts},
		RootCAs:      caCertPool,
	}

	stsTR := &http.Transport{
		TLSClientConfig: stsTLSConfig,
	}
	stsClient := &http.Client{Transport: stsTR}

	form := url.Values{}
	form.Add("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Add("resource", *stsAudience)
	form.Add("audience", *stsAudience)
	form.Add("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	form.Add("requested_token_type", "urn:ietf:params:oauth:token-type:jwt")
	form.Add("scope", *stsScope)
	form.Add("subject_token", *stsCred)

	stsResp, err := stsClient.PostForm(*stsAddress, form)
	if err != nil {
		fmt.Printf("Error: could not connect %v", err)
		return
	}
	defer stsResp.Body.Close()

	if stsResp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(stsResp.Body)
		fmt.Printf("Unable to exchange token %s,  %v", string(bodyBytes), err)
		return
	}

	body, err := ioutil.ReadAll(stsResp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%v\n", stsResp.Status)

	var result TokenResponse
	if err := json.Unmarshal(body, &result); err != nil { // Parse []byte to the go struct pointer
		fmt.Println("Can not unmarshal JSON")
		return
	}

	fmt.Printf("STS Token: %s\n", result.AccessToken)
	// ************************************************

	req, err := http.NewRequest("GET", *resourceAddress, nil)
	if err != nil {
		fmt.Println("Can not get Resource")
		return
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", result.AccessToken))

	resourceTLSConfig := &tls.Config{
		ServerName:   *resourceSNI,
		Certificates: []tls.Certificate{clientCerts},
		RootCAs:      caCertPool,
	}

	resourceTR := &http.Transport{
		TLSClientConfig: resourceTLSConfig,
	}

	client := &http.Client{
		Transport: resourceTR,
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error on response.\n[ERROR] %v\n", err)
		return
	}
	defer resp.Body.Close()

	resourceBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error while reading the response bytes: %v\n", err)
		return
	}
	log.Println(string([]byte(resourceBody)))

	// ***********************************

	// The following is just a demo of what woudl happen if you
	// uses alice's STS JWT token (the one the STS server returned)
	// with a client certificate for Bob (this is expected to fail)

	// otherCerts, err := tls.LoadX509KeyPair(
	// 	"../certs/bob.crt",
	// 	"../certs/bob.key",
	// )
	// if err != nil {
	// 	fmt.Printf("Error reading otherCerts .\n[ERROR] %v\n", err)
	// 	return
	// }

	// resourceTLSConfig = &tls.Config{
	// 	ServerName:   *resourceSNI,
	// 	Certificates: []tls.Certificate{otherCerts},
	// 	RootCAs:      caCertPool,
	// }

	// resourceTR = &http.Transport{
	// 	TLSClientConfig: resourceTLSConfig,
	// }

	// client = &http.Client{
	// 	Transport: resourceTR,
	// }
	// resp, err = client.Do(req)
	// if err != nil {
	// 	fmt.Printf("Error on response.\n[ERROR] %v\n", err)
	// 	return
	// }
	// defer resp.Body.Close()

	// resourceBody, err = ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	fmt.Printf("Error while reading the response bytes: %v\n", err)
	// 	return
	// }
	// log.Println(string([]byte(resourceBody)))
}
