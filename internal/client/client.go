package client

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Dynamic dynamic.Interface
}

func NewFromDefault() (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading default kubeconfig: %w", err)
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	return &Client{Dynamic: dyn}, nil
}

func NewFromContext(kubeconfig, ctx string) (*Client, error) {
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if ctx != "" {
		overrides.CurrentContext = ctx
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig %s (context %s): %w", kubeconfig, ctx, err)
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	return &Client{Dynamic: dyn}, nil
}

func (c *Client) resource(gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace != "" {
		return c.Dynamic.Resource(gvr).Namespace(namespace)
	}
	return c.Dynamic.Resource(gvr)
}

func (c *Client) Create(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.resource(gvr, namespace).Create(ctx, obj, metav1.CreateOptions{})
}

func (c *Client) Get(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	return c.resource(gvr, namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Client) List(ctx context.Context, gvr schema.GroupVersionResource, namespace, labelSelector string) (*unstructured.UnstructuredList, error) {
	opts := metav1.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}
	return c.resource(gvr, namespace).List(ctx, opts)
}

func (c *Client) Delete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	return c.resource(gvr, namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Client) Patch(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, pt types.PatchType, data []byte) (*unstructured.Unstructured, error) {
	return c.resource(gvr, namespace).Patch(ctx, name, pt, data, metav1.PatchOptions{})
}

func (c *Client) Watch(ctx context.Context, gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.resource(gvr, namespace).Watch(ctx, opts)
}
