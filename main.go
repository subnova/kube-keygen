package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"github.com/subnova/kube-genkey/ssh"
	"log"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/rest"
	"io/ioutil"
)

var (
	keyNames = kingpin.Flag("key-name", "Name of key to create SSH key for").Strings()
	sshKeyGenDir = kingpin.Flag("ssh-keygen-dir", "Tempfs directory to use when generating keys").Required().String()
	kubeconfig = kingpin.Flag("kubeconfig", "Kubernetes configuration to use for out-of-cluster operation").String()
	master = kingpin.Flag("master", "The address of Kubernetes API server for use in out-of-cluster operation").String()
)

func main() {
	kingpin.Version("0.1.0")
	kingpin.Parse()

	var cfg *rest.Config
	var err error
	var namespace string

	if *kubeconfig == "" && *master == "" {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Unable to connect to API server: %v", err)
		}

		namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			log.Fatalf("Unable to determine current namespace: %v", err)
		}
		namespace = string(namespaceBytes)
	} else {
		clientcfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig},
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: *master}},
		)

		namespace, _, err = clientcfg.Namespace()
		if err != nil {
			log.Fatalf("Unable to determine namespace: %v", err)
		}

		cfg, err = clientcfg.ClientConfig()
		if err != nil {
			log.Fatalf("Unable to connect to API server: %w", err)
		}
	}


	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %v", err)
	}

	for _, keyName := range *keyNames {
		privateName := keyName + "-ssh"
		publicName := privateName + ".pub"
		secrets, err := kubeClient.CoreV1().Secrets(namespace).List(meta_v1.SingleObject(meta_v1.ObjectMeta{Name: privateName}))
		if err != nil {
			log.Printf("Unable to retrieve existing secret for %s", keyName)
			continue
		}

		if len(secrets.Items) == 0 {
			log.Printf("Creating key %s", keyName)
			_, privateKey, publicKey, err := ssh.KeyGen(*sshKeyGenDir)
			if err != nil {
				log.Fatalf("Unable to generate key %s: %q", keyName, err)
			}

			kubeClient.CoreV1().Secrets(namespace).Create(&v1.Secret{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: privateName,
					Namespace: namespace,
				},
				Data: map[string][]byte {
					privateName: privateKey,
				},
			})

			kubeClient.CoreV1().ConfigMaps(namespace).Create(&v1.ConfigMap{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: publicName,
					Namespace: namespace,
				},
				Data: map[string]string {
					publicName: publicKey.Key,
				},
			})
		} else {
			log.Printf("Key already exists for %s", keyName)
		}
	}
}

