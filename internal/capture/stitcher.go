package capture

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PodTarget tracks a capture subject with optional workload label selector for restarts.
type PodTarget struct {
	Namespace     string
	PodName       string
	PodUID        string
	LabelSelector string
	Port          int32
}

// Stitcher maintains a single stdout byte stream across pod replacements.
type Stitcher struct {
	client   kubernetes.Interface
	runner   *NetObservRunner
	target   PodTarget
	mu       sync.Mutex
	cancel   context.CancelFunc
	reader   io.Reader
	onSwitch func(newPod PodTarget, stitched bool)
}

func NewStitcher(client kubernetes.Interface, runner *NetObservRunner, target PodTarget) *Stitcher {
	return &Stitcher{client: client, runner: runner, target: target}
}

func (s *Stitcher) OnSwitch(fn func(PodTarget, bool)) {
	s.onSwitch = fn
}

func (s *Stitcher) Start(ctx context.Context) (io.Reader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, cancel, err := s.runner.Start(ctx, s.target.Namespace, s.target.PodName, s.target.Port)
	if err != nil {
		return nil, fmt.Errorf("failed tracking target pod state: %w", err)
	}
	s.reader = r
	s.cancel = cancel
	return s.reader, nil
}

func (s *Stitcher) WatchRestarts(ctx context.Context) error {
	if s.target.LabelSelector == "" {
		return nil
	}

	if _, err := labels.Parse(s.target.LabelSelector); err != nil {
		return fmt.Errorf("invalid label selector for stitcher: %w", err)
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		s.client,
		time.Second*30,
		informers.WithNamespace(s.target.Namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = s.target.LabelSelector
		}),
	)

	informer := factory.Core().V1().Pods().Informer()
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok || pod == nil {
				return
			}
			if string(pod.UID) == s.target.PodUID || pod.Name == s.target.PodName {
				return
			}
			if pod.Status.Phase != corev1.PodRunning {
				return
			}
			s.switchPod(ctx, pod)
		},
		UpdateFunc: func(_, newObj interface{}) {
			pod, ok := newObj.(*corev1.Pod)
			if !ok || pod == nil {
				return
			}
			if string(pod.UID) == s.target.PodUID {
				return
			}
			if pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil {
				s.switchPod(ctx, pod)
			}
		},
	})

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("pod informer cache failed to sync for namespace %s", s.target.Namespace)
	}

	// Also watch direct pod deletion by name
	go s.watchNamedPod(ctx)

	<-ctx.Done()
	return ctx.Err()
}

func (s *Stitcher) watchNamedPod(ctx context.Context) {
	watcher, err := s.client.CoreV1().Pods(s.target.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", s.target.PodName).String(),
	})
	if err != nil {
		return
	}
	defer watcher.Stop()

	for ev := range watcher.ResultChan() {
		pod, ok := ev.Object.(*corev1.Pod)
		if !ok {
			continue
		}
		if ev.Type == "DELETED" || pod.Status.Phase == corev1.PodFailed {
			replacement, err := s.findReplacement(ctx)
			if err == nil && replacement != nil {
				s.switchPod(ctx, replacement)
			}
		}
	}
}

func (s *Stitcher) findReplacement(ctx context.Context) (*corev1.Pod, error) {
	if s.target.LabelSelector == "" {
		return nil, fmt.Errorf("no label selector to find replacement pod")
	}
	list, err := s.client.CoreV1().Pods(s.target.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: s.target.LabelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed listing replacement pods: %w", err)
	}
	for i := range list.Items {
		p := &list.Items[i]
		if p.Status.Phase == corev1.PodRunning && string(p.UID) != s.target.PodUID {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no running replacement pod found")
}

func (s *Stitcher) switchPod(ctx context.Context, pod *corev1.Pod) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if string(pod.UID) == s.target.PodUID && pod.Name == s.target.PodName {
		return
	}

	if s.cancel != nil {
		s.cancel()
	}

	newTarget := PodTarget{
		Namespace:     s.target.Namespace,
		PodName:       pod.Name,
		PodUID:        string(pod.UID),
		LabelSelector: s.target.LabelSelector,
		Port:          s.target.Port,
	}

	r, cancel, err := s.runner.Start(ctx, newTarget.Namespace, newTarget.PodName, newTarget.Port)
	if err != nil {
		return
	}

	s.target = newTarget
	s.reader = r
	s.cancel = cancel

	if s.onSwitch != nil {
		s.onSwitch(newTarget, true)
	}
}

func (s *Stitcher) CurrentTarget() PodTarget {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.target
}

func (s *Stitcher) ActiveReader() io.Reader {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reader
}
