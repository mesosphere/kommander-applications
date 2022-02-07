package main

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type Registry struct {
	objects    map[ObjectRef]interface{}
	checked    map[ObjectRef]bool
	crdSchemas map[metav1.TypeMeta]*spec.Schema
	sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		objects:    make(map[ObjectRef]interface{}),
		checked:    make(map[ObjectRef]bool),
		crdSchemas: make(map[metav1.TypeMeta]*spec.Schema),
	}
}

func (r *Registry) SetObject(ref ObjectRef, obj interface{}) {
	r.Lock()
	r.objects[ref] = obj
	r.Unlock()
}

func (r *Registry) GetObject(ref ObjectRef) interface{} {
	r.RLock()
	defer r.RUnlock()
	return r.objects[ref]
}

func (r *Registry) MarkChecked(ref ObjectRef) {
	r.Lock()
	r.checked[ref] = true
	r.Unlock()
}

func (r *Registry) IsChecked(ref ObjectRef) bool {
	r.RLock()
	defer r.RUnlock()
	return r.checked[ref]
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

type ObjectRef struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        ObjectRefMeta `json:"metadata,omitempty"`
}

type ObjectRefMeta struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

func NewObjectRef(apiVersion, kind, namespace, name string) ObjectRef {
	return ObjectRef{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		Metadata: ObjectRefMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func metasToRef(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta) ObjectRef {
	return ObjectRef{
		TypeMeta: typeMeta,
		Metadata: ObjectRefMeta{
			Name:      objectMeta.Name,
			Namespace: objectMeta.Namespace,
		},
	}
}
