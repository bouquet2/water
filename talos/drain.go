package talos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultDrainTimeout = 5 * time.Minute
)

type DrainHelper struct {
	client   kubernetes.Interface
	nodeName string
	timeout  time.Duration
}

func NewDrainHelper(client kubernetes.Interface, nodeName string, timeout time.Duration) *DrainHelper {
	return &DrainHelper{
		client:   client,
		nodeName: nodeName,
		timeout:  timeout,
	}
}

func (d *DrainHelper) Cordon(ctx context.Context) error {
	log.Info().Str("node", d.nodeName).Msg("Cordoning node")

	node, err := d.client.CoreV1().Nodes().Get(ctx, d.nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", d.nodeName, err)
	}

	if node.Spec.Unschedulable {
		log.Debug().Str("node", d.nodeName).Msg("Node already cordoned")
		return nil
	}

	node.Spec.Unschedulable = true
	_, err = d.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to cordon node %s: %w", d.nodeName, err)
	}

	log.Info().Str("node", d.nodeName).Msg("Node cordoned successfully")
	return nil
}

func (d *DrainHelper) Uncordon(ctx context.Context) error {
	log.Info().Str("node", d.nodeName).Msg("Uncordoning node")

	node, err := d.client.CoreV1().Nodes().Get(ctx, d.nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", d.nodeName, err)
	}

	if !node.Spec.Unschedulable {
		log.Debug().Str("node", d.nodeName).Msg("Node already uncordoned")
		return nil
	}

	node.Spec.Unschedulable = false
	_, err = d.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to uncordon node %s: %w", d.nodeName, err)
	}

	log.Info().Str("node", d.nodeName).Msg("Node uncordoned successfully")
	return nil
}

func (d *DrainHelper) EvictPods(ctx context.Context) error {
	log.Info().Str("node", d.nodeName).Msg("Evicting pods from node")

	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	pods, err := d.client.CoreV1().Pods(metav1.NamespaceAll).List(timeoutCtx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", d.nodeName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods on node %s: %w", d.nodeName, err)
	}

	var errs []error
	for _, pod := range pods.Items {
		if err := d.evictPod(timeoutCtx, &pod); err != nil {
			log.Warn().
				Str("pod", pod.Name).
				Str("namespace", pod.Namespace).
				Err(err).
				Msg("Failed to evict pod")
			errs = append(errs, fmt.Errorf("pod %s/%s: %w", pod.Namespace, pod.Name, err))
			continue
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to evict %d pods: %w", len(errs), errors.Join(errs...))
	}

	log.Info().Str("node", d.nodeName).Msg("Pods evicted successfully")
	return nil
}

func (d *DrainHelper) evictPod(ctx context.Context, pod *corev1.Pod) error {
	if d.isDaemonSetOrMirrorPod(pod) {
		log.Debug().
			Str("pod", pod.Name).
			Str("namespace", pod.Namespace).
			Msg("Skipping DaemonSet or mirror pod")
		return nil
	}

	hasEmptyDir := d.hasEmptyDir(pod)
	if hasEmptyDir {
		log.Warn().
			Str("pod", pod.Name).
			Str("namespace", pod.Namespace).
			Msg("Pod has emptyDir volumes - data will be lost upon eviction")
	}

	gracePeriodSeconds := int64(30)
	if pod.Spec.TerminationGracePeriodSeconds != nil {
		gracePeriodSeconds = *pod.Spec.TerminationGracePeriodSeconds
	}

	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriodSeconds,
		},
	}

	log.Info().
		Str("pod", pod.Name).
		Str("namespace", pod.Namespace).
		Int64("gracePeriod", gracePeriodSeconds).
		Bool("hasEmptyDir", hasEmptyDir).
		Msg("Evicting pod")

	err := d.client.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction)
	if err != nil {
		return fmt.Errorf("failed to evict pod: %w", err)
	}

	log.Info().
		Str("pod", pod.Name).
		Str("namespace", pod.Namespace).
		Msg("Pod evicted successfully")

	return nil
}

func (d *DrainHelper) isDaemonSetOrMirrorPod(pod *corev1.Pod) bool {
	for _, ownerRef := range pod.ObjectMeta.OwnerReferences {
		if ownerRef.Kind == "DaemonSet" {
			return true
		}
	}

	if pod.Annotations != nil {
		if _, exists := pod.Annotations["kubernetes.io/mirror"]; exists {
			return true
		}
	}

	return false
}

func (d *DrainHelper) hasEmptyDir(pod *corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.EmptyDir != nil {
			return true
		}
	}
	return false
}

func DrainNode(ctx context.Context, k8sClient kubernetes.Interface, nodeName string, timeout time.Duration) error {
	log.Info().
		Str("node", nodeName).
		Dur("timeout", timeout).
		Msg("Starting node drain")

	helper := NewDrainHelper(k8sClient, nodeName, timeout)

	if err := helper.Cordon(ctx); err != nil {
		return fmt.Errorf("failed to cordon node: %w", err)
	}

	if err := helper.EvictPods(ctx); err != nil {
		return fmt.Errorf("failed to evict pods: %w", err)
	}

	log.Info().Str("node", nodeName).Msg("Node drain completed successfully")
	return nil
}

func UncordonNode(ctx context.Context, k8sClient kubernetes.Interface, nodeName string) error {
	log.Info().Str("node", nodeName).Msg("Uncordoning node")

	helper := NewDrainHelper(k8sClient, nodeName, 0)

	if err := helper.Uncordon(ctx); err != nil {
		return fmt.Errorf("failed to uncordon node: %w", err)
	}

	log.Info().Str("node", nodeName).Msg("Node uncordoned successfully")
	return nil
}
