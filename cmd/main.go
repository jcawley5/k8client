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
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func main() {

	namespace := "default3"

	var kubeconfig *string
	var filename *string
	if dir, _ := os.Getwd(); dir != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(dir, "../kubeconfig.yml"), "(optional) absolute path to the kubeconfig file")
		filename = flag.String("filename", filepath.Join(dir, "../deployment.yaml"), "")
		fmt.Println(kubeconfig)
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
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

	err = createNamespace(clientset, namespace)
	if err != nil {
		log.Fatal(err)
	}

	applyDeployment(clientset, dynamicConfig, filename, namespace)

}

func createNamespace(clientset *kubernetes.Clientset, namespace string) error {
	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}

	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), nsSpec, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	return nil
}

func applyDeployment(clientset *kubernetes.Clientset, dynamicConfig dynamic.Interface, filename *string, namespace string) {
	deployment, err := ioutil.ReadFile(*filename)
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
				unstructuredObj.SetNamespace(namespace)
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
