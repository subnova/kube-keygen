package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"github.com/subnova/kube-keygen/ssh"
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
	sshKeyGenDir = kingpin.Flag("ssh-keygen-dir", "Tempfs directory to use when generating keys").Required().String()
	repoNames    = kingpin.Flag("repo-name", "Name of repository to create SSH key pair for").Strings()

	kubeconfig   = kingpin.Flag("kubeconfig", "Kubernetes configuration to use for out-of-cluster operation").String()
	master       = kingpin.Flag("master", "The address of Kubernetes API server for use in out-of-cluster operation").String()
)

func main() {
	kingpin.Version("0.1.0")
	kingpin.Parse()

	var cfg *rest.Config
	var err error
	var namespace string

	// connect to kube
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

	// generate key pairs
	for _, repoName := range *repoNames {
		privateName := repoName + "-ssh"
		publicName := privateName + ".pub"
		secrets, err := kubeClient.CoreV1().Secrets(namespace).List(meta_v1.SingleObject(meta_v1.ObjectMeta{Name: privateName}))
		if err != nil {
			log.Printf("Unable to retrieve existing secret for %s", repoName)
			continue
		}

		if len(secrets.Items) == 0 {
			log.Printf("Creating key %s", repoName)
			_, privateKey, publicKey, err := ssh.KeyGen(*sshKeyGenDir)
			if err != nil {
				log.Fatalf("Unable to generate key %s: %q", repoName, err)
			}

			_, err = kubeClient.CoreV1().Secrets(namespace).Create(&v1.Secret{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: privateName,
					Namespace: namespace,
				},
				Data: map[string][]byte {
					"identity": privateKey,
				},
			})
			if err != nil {
				log.Fatalf("Unable to add secret %s/%s: %v", namespace, privateName, err)
			}

			_, err = kubeClient.CoreV1().ConfigMaps(namespace).Create(&v1.ConfigMap{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: publicName,
					Namespace: namespace,
				},
				Data: map[string]string {
					"publicKey": publicKey.Key,
				},
			})
			if err != nil {
				log.Fatalf("Unable to add configmap %s/%s: %v", namespace, publicName, err)
			}
		} else {
			log.Printf("Key already exists for %s", repoName)
		}
	}

	// generate base configuration
	knownHostKeys, err := ssh.KeyScan([]string{"github.com"})
	if err != nil {
		log.Fatalf("Unable to obtain known host keys: %v", knownHostKeys)
	}

	configMaps, err := kubeClient.CoreV1().ConfigMaps(namespace).List(meta_v1.SingleObject(meta_v1.ObjectMeta{Name: "ssh-setup"}))
	if err != nil {
		log.Fatalf("Error listing configmaps: %v", err)
	}

	sshConfig := ssh.Config(*repoNames)

	configMap := &v1.ConfigMap{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "ssh-setup",
			Namespace: namespace,
		},
		Data: map[string]string{
			"known_hosts": knownHostKeys,
			"config": sshConfig,
		},
	}

	if len(configMaps.Items) == 0 {
		_, err = kubeClient.CoreV1().ConfigMaps(namespace).Create(configMap)
		if err != nil {
			log.Fatalf("Unable to update configmap %s/%s: %v", namespace, "known_hosts", err)
		}
	} else {
		_, err = kubeClient.CoreV1().ConfigMaps(namespace).Update(configMap)
		if err != nil {
			log.Fatalf("Unable to update configmap %s/%s: %v", namespace, "known_hosts", err)
		}
	}

}

