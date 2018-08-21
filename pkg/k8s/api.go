package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var minApiVersion = [3]int{1, 8, 0}

type KubernetesAPI struct {
	*rest.Config
}

func (kubeAPI *KubernetesAPI) NewClient() (*http.Client, error) {
	secureTransport, err := rest.TransportFor(kubeAPI.Config)
	if err != nil {
		return nil, fmt.Errorf("error instantiating Kubernetes API client: %v", err)
	}

	return &http.Client{
		Transport: secureTransport,
	}, nil
}

func (kubeAPI *KubernetesAPI) GetVersionInfo(client *http.Client) (*version.Info, error) {
	endpoint, err := url.Parse(kubeAPI.Host + "/version")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rsp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected Kubernetes API response: %s", rsp.Status)
	}

	bytes, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	var versionInfo version.Info
	err = json.Unmarshal(bytes, &versionInfo)
	return &versionInfo, err
}

func (kubeAPI *KubernetesAPI) CheckVersion(versionInfo *version.Info) error {
	apiVersion, err := getK8sVersion(versionInfo.String())
	if err != nil {
		return err
	}

	if !isCompatibleVersion(minApiVersion, apiVersion) {
		return fmt.Errorf("Kubernetes is on version [%d.%d.%d], but version [%d.%d.%d] or more recent is required",
			apiVersion[0], apiVersion[1], apiVersion[2],
			minApiVersion[0], minApiVersion[1], minApiVersion[2])
	}

	return nil
}

func (kubeAPI *KubernetesAPI) NamespaceExists(client *http.Client, namespace string) (bool, error) {
	endpoint, err := url.Parse(kubeAPI.Host + "/api/v1/namespaces/" + namespace)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rsp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return false, err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK && rsp.StatusCode != http.StatusNotFound {
		return false, fmt.Errorf("Unexpected Kubernetes API response: %s", rsp.Status)
	}

	return rsp.StatusCode == http.StatusOK, nil
}

// UrlFor generates a URL based on the Kubernetes config.
func (kubeAPI *KubernetesAPI) UrlFor(namespace string, extraPathStartingWithSlash string) (*url.URL, error) {
	return generateKubernetesApiBaseUrlFor(kubeAPI.Host, namespace, extraPathStartingWithSlash)
}

// NewAPI validates a Kubernetes config and returns a client for accessing the
// configured cluster
func NewAPI(configPath string) (*KubernetesAPI, error) {
	config, err := getConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("error configuring Kubernetes API client: %v", err)
	}

	return &KubernetesAPI{Config: config}, nil
}
