package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func pingHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("pong\n"))
	logrus.Debugf("handling request %s\n", req.URL.Path)
}

func collectionHandler(nsClient corev1.NamespaceInterface, dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Debugf("handling request %s", r.URL.Path)
		vars := mux.Vars(r)
		logrus.Debugf("resource: %s", vars["resource"])
		logrus.Debugf("groupversion: %s", vars["groupversion"])
		logrus.Debugf("project: %s", vars["project"])
		nss, err := nsClient.List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("field.cattle.io/projectId=%s", vars["project"]),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		groupVersion := strings.SplitN(vars["groupversion"], ".", 2)
		version := groupVersion[0]
		group := ""
		if len(groupVersion) > 1 {
			group = groupVersion[1]
		}
		resource := schema.GroupVersionResource{Group: group, Version: version, Resource: vars["resource"]}
		resourceClient := dynamicClient.Resource(resource)
		resourceCollection := make([]*unstructured.UnstructuredList, 0)
		for _, n := range nss.Items {
			resourcesForNamespace, err := resourceClient.Namespace(n.Name).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			resourceCollection = append(resourceCollection, resourcesForNamespace)
		}
		resourceJSON, err := json.Marshal(resourceCollection)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resourceJSON)
	}
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.Info("starting server")
	c, err := getClient()
	if err != nil {
		logrus.Fatal(err)
		return
	}
	coreClient, err := corev1.NewForConfig(c)
	if err != nil {
		logrus.Fatal(err)
		return
	}
	nsClient := coreClient.Namespaces()
	dynamicClient, err := dynamic.NewForConfig(c)
	if err != nil {
		logrus.Fatal(err)
		return
	}
	r := mux.NewRouter()
	r.HandleFunc("/apis/projects.cattle.io/v1alpha1", pingHandler)
	r.HandleFunc("/apis/projects.cattle.io/v1alpha1/{groupversion}/{resource}/{project}", collectionHandler(nsClient, dynamicClient))
	http.Handle("/", r)
	err = http.ListenAndServeTLS(":4444", "examples/cert.pem", "examples/cert.key", nil)
	if err != nil {
		logrus.Fatal("ListenAndServe: ", err)
	}
}

func getClient() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil).ClientConfig()
}
