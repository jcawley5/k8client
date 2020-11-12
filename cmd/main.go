package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

type config struct {
	kubeconfig *string
	filename   *string
	namespace  string
}

func main() {

	var c config
	c.initConfig()

	// SET CONFIG...
	// in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Println(err.Error())

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *c.kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	dynamicConfig, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	err = c.createNamespace(clientset)
	if err != nil {
		log.Fatal(err)
	}

	c.applyDeployment(clientset, dynamicConfig)

}

func (c *config) createNamespace(clientset *kubernetes.Clientset) error {
	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.namespace}}

	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), nsSpec, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	return nil
}

func (c *config) applyDeployment(clientset *kubernetes.Clientset, dynamicConfig dynamic.Interface) {
	deployment, err := ioutil.ReadFile(*c.filename)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%q \n", string(deployment))

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(deployment), 100)

	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			log.Fatal(err)
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

		gr, err := restmapper.GetAPIGroupResources(clientset.Discovery())
		if err != nil {
			log.Fatal(err)
		}

		mapper := restmapper.NewDiscoveryRESTMapper(gr)
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			log.Fatal(err)
		}

		var dri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if unstructuredObj.GetNamespace() == "" {
				unstructuredObj.SetNamespace(c.namespace)
			}
			//set unique host
			if unstructuredObj.GetKind() == "APIRule" {
				if err := unstructured.SetNestedField(unstructuredObj.Object, "nginx-"+c.namespace, "spec", "service", "host"); err != nil {
					panic(err)
				}
			}
			dri = dynamicConfig.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			dri = dynamicConfig.Resource(mapping.Resource)
		}

		if _, err := dri.Create(context.Background(), unstructuredObj, metav1.CreateOptions{}); err != nil {
			log.Fatal(err)
		}
		fmt.Print(unstructuredObj)
	}
	if err != io.EOF {
		log.Fatal("eof ", err)
	}
}

//set the config values to be used in the request
func (c *config) initConfig() {

	dir, err := os.Getwd()
	if err != nil {
		log.Panic("could not get directory path")
	}

	c.kubeconfig = flag.String("kubeconfig", filepath.Join(dir, "./kubeconfig.yml"), "")
	c.filename = flag.String("filename", filepath.Join(dir, "./deployment.yaml"), "")
	flag.StringVar(&c.namespace, "namespace", "", "the namespace to create and deploy to")

	flag.Parse()

}
