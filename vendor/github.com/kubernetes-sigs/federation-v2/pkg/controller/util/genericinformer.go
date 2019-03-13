/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func NewGenericInformer(config *rest.Config, namespace string, obj pkgruntime.Object, triggerFunc func(pkgruntime.Object)) (cache.Store, cache.Controller, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
	if err != nil {
		return nil, nil, err
	}

	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Could not create RESTMapper from config")
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, nil, err
	}

	client, err := apiutil.RESTClientForGVK(gvk, config, scheme.Codecs)
	if err != nil {
		return nil, nil, err
	}

	listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
	listObj, err := scheme.Scheme.New(listGVK)
	if err != nil {
		return nil, nil, err
	}

	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (pkgruntime.Object, error) {
				res := listObj.DeepCopyObject()
				isNamespaceScoped := namespace != "" && mapping.Scope.Name() != meta.RESTScopeNameRoot
				err := client.Get().NamespaceIfScoped(namespace, isNamespaceScoped).Resource(mapping.Resource.Resource).VersionedParams(&opts, scheme.ParameterCodec).Do().Into(res)
				return res, err
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				// Watch needs to be set to true separately
				opts.Watch = true
				isNamespaceScoped := namespace != "" && mapping.Scope.Name() != meta.RESTScopeNameRoot
				return client.Get().NamespaceIfScoped(namespace, isNamespaceScoped).Resource(mapping.Resource.Resource).VersionedParams(&opts, scheme.ParameterCodec).Watch()
			},
		},
		obj,
		NoResyncPeriod,
		NewTriggerOnAllChanges(triggerFunc),
	)
	return store, controller, nil
}
