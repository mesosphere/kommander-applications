package main

import (
	"sync"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type Registry struct {
	objects    map[meta.NamespacedObjectKindReference]interface{}
	checked    map[meta.NamespacedObjectKindReference]bool
	valid      map[meta.NamespacedObjectKindReference]bool
	crdSchemas map[metav1.TypeMeta]*spec.Schema
	sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		objects:    make(map[meta.NamespacedObjectKindReference]interface{}),
		checked:    make(map[meta.NamespacedObjectKindReference]bool),
		valid:      make(map[meta.NamespacedObjectKindReference]bool),
		crdSchemas: make(map[metav1.TypeMeta]*spec.Schema),
	}
}

func (r *Registry) SetData(ref meta.NamespacedObjectKindReference, obj interface{}) {
	r.Lock()
	r.objects[ref] = obj
	r.Unlock()
}

func (r *Registry) GetData(ref meta.NamespacedObjectKindReference) interface{} {
	r.RLock()
	defer r.RUnlock()
	return r.objects[ref]
}

func (r *Registry) MarkChecked(ref meta.NamespacedObjectKindReference) {
	r.Lock()
	r.checked[ref] = true
	r.Unlock()
}

func (r *Registry) IsChecked(ref meta.NamespacedObjectKindReference) bool {
	r.RLock()
	defer r.RUnlock()
	return r.checked[ref]
}

func (r *Registry) MarkValid(ref meta.NamespacedObjectKindReference) {
	r.Lock()
	r.valid[ref] = true
	r.Unlock()
}

func (r *Registry) IsValid(ref meta.NamespacedObjectKindReference) bool {
	r.RLock()
	defer r.RUnlock()
	return r.valid[ref]
}

func (r *Registry) SetCRDSchema(typeMeta metav1.TypeMeta, schema *spec.Schema) {
	r.Lock()
	r.crdSchemas[typeMeta] = schema
	r.Unlock()
}

func (r *Registry) GetCRDSchema(typeMeta metav1.TypeMeta) *spec.Schema {
	r.RLock()
	defer r.RUnlock()
	return r.crdSchemas[typeMeta]
}

func metasToRef(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta) meta.NamespacedObjectKindReference {
	return meta.NamespacedObjectKindReference{
		APIVersion: typeMeta.APIVersion,
		Kind:       typeMeta.Kind,
		Namespace:  objectMeta.Namespace,
		Name:       objectMeta.Name,
	}
}
