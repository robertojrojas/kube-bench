package cmd

import (
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
)

func getKubeVersionFromRESTAPI() (string, error) {
	k8sVersionURL := "https://kubernetes.default.svc/version"
	serviceaccount := "/var/run/secrets/kubernetes.io/serviceaccount"

	token, cacertfile, err := readTokenAndCertfile(serviceaccount)
	if err != nil {
		return "", err
	}

	k8sVersion, err := getK8SVersion(k8sVersionURL, string(token), cacertfile)
	if err != nil {
		return "", err
	}
	return k8sVersion, nil
}

func getK8SVersion(k8sVersionURL, token string, cacert []byte) (string, error) {
	glog.V(2).Info(fmt.Sprintf("getK8SVersion URL: %s\n", k8sVersionURL))
	/*
		{
		  "major": "1",
		  "minor": "15",
		  "gitVersion": "v1.15.3",
		  "gitCommit": "2d3c76f9091b6bec110a5e63777c332469e0cba2",
		  "gitTreeState": "clean",
		  "buildDate": "2019-08-20T18:57:36Z",
		  "goVersion": "go1.12.9",
		  "compiler": "gc",
		  "platform": "linux/amd64"
		}
	*/
	type versionResponse struct {
		Major        string
		Minor        string
		GitVersion   string
		GitCommit    string
		GitTreeState string
		BuildDate    string
		GoVersion    string
		Compiler     string
		Platform     string
	}

	vd, err := getWebData(k8sVersionURL, token, cacert)
	if err != nil {
		return "", err
	}

	vrObj := &versionResponse{}
	glog.V(2).Info(fmt.Sprintf("vd: %s\n", string(vd)))
	err = json.Unmarshal(vd, vrObj)
	if err != nil {
		return "", err
	}
	glog.V(2).Info(fmt.Sprintf("vrObj: %#v\n", vrObj))

	// Some provides return the minor version like "15+"
	minor := strings.Replace(vrObj.Minor, "+", "", -1)
	ver := fmt.Sprintf("%s.%s", vrObj.Major, minor)
	return ver, nil
}

func readTokenAndCertfile(saDir string) ([]byte, []byte, error) {
	_, err := os.Stat(saDir)
	if os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("missing service account directory: %q", saDir)
	}

	cacertfile := fmt.Sprintf("%s/ca.crt", saDir)
	cacertdata, err := ioutil.ReadFile(cacertfile)
	if err != nil {
		return nil, nil, err
	}

	tfile := fmt.Sprintf("%s/token", saDir)
	token, err := ioutil.ReadFile(tfile)

	if err != nil {
		return nil, nil, err
	}

	return token, cacertdata, nil
}

func getWebData(srvURL, token string, cacert []byte) ([]byte, error) {
	glog.V(2).Info(fmt.Sprintf("getWebData srvURL: %s\n", srvURL))
	cert, err := loadCertficate(cacert)
	if err != nil {
		return nil, err
	}

	tlsConf := &tls.Config{
		Certificates:       []tls.Certificate{*cert},
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{
		TLSClientConfig: tlsConf,
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", srvURL, nil)
	if err != nil {
		return nil, err
	}

	token = strings.TrimSpace(token)

	authToken := fmt.Sprintf("Bearer %s", token)
	glog.V(2).Info(fmt.Sprintf("getWebData AUTH TOKEN --[%q]--\n", authToken))
	req.Header.Set("Authorization", authToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("HTTP ERROR: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		glog.V(2).Info(fmt.Sprintf("URL:[%s], StatusCode:[%d] \n Headers: %#v\n", srvURL, resp.StatusCode, resp.Header))
		err = fmt.Errorf("URL:[%s], StatusCode:[%d]", srvURL, resp.StatusCode)
		return nil, err

	}

	return ioutil.ReadAll(resp.Body)
}

func loadCertficate(raw []byte) (*tls.Certificate, error) {
	var cert tls.Certificate

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("unable to Decode certificate")
	}

	glog.V(2).Info(fmt.Sprintf("Loading CA certificate"))
	cert.Certificate = append(cert.Certificate, block.Bytes)
	return &cert, nil
}
