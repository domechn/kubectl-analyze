package podusage

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/domgoer/kubectl-analyze/pkg/tabwriter"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type UsageLister struct {
	client        kubernetes.Interface
	metricsClient metricsclientset.Interface
	writer        tabwriter.Writer
}

type usageShowData struct {
	namespace string
	name      string

	requestCPU    *resource.Quantity
	requestMemory *resource.Quantity
	limitMemory   *resource.Quantity
	limitCPU      *resource.Quantity

	usageCPU              *resource.Quantity
	usageCPUPercentage    string
	usageMemory           *resource.Quantity
	usageMemroyPercentage string
}

func MustNew(restConfig *rest.Config) *UsageLister {
	restConfig.Burst = math.MaxInt64
	mcli := metricsclientset.NewForConfigOrDie(restConfig)
	kcli := kubernetes.NewForConfigOrDie(restConfig)

	writer := tabwriter.New(os.Stdout)
	return &UsageLister{

		client:        kcli,
		metricsClient: mcli,
		writer:        writer,
	}
}

func (l *UsageLister) getPods(name, namespace, nodeName string) ([]corev1.Pod, error) {
	if name == "" && namespace == "" && nodeName == "" {
		return nil, fmt.Errorf("must set search options")
	}
	if name != "" && namespace != "" {
		pod, err := l.client.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return []corev1.Pod{*pod}, nil
	}
	listOptions := metav1.ListOptions{}
	if nodeName != "" {
		listOptions.FieldSelector = fmt.Sprintf("spec.nodeName=%s", nodeName)
	}
	pods, err := l.client.CoreV1().Pods(namespace).List(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (l *UsageLister) FindUsageNotMatchRequest(name, namespace, nodeName string, multiple float64) ([]*usageShowData, error) {
	pods, err := l.getPods(name, namespace, nodeName)
	if err != nil {
		return nil, err
	}

	filter := func(pod corev1.Pod) (*usageShowData, error) {
		metrics, err := l.metricsClient.MetricsV1beta1().PodMetricses(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}

		for idx, container := range pod.Spec.Containers {
			limit := container.Resources.Limits
			req := container.Resources.Requests
			reqMem := req.Memory().Value()
			reqCPU := req.Cpu().MilliValue()
			usage := metrics.Containers[idx].Usage
			usageMem := usage.Memory().Value()
			usageCPU := usage.Cpu().MilliValue()

			usageMemPercentage, usageMemPercentageShow := beautyUsage(float64(usageMem), float64(reqMem))
			usageCPUPercentage, usageCPUPercentageShow := beautyUsage(float64(usageCPU), float64(reqCPU))
			b := (reqMem == 0 || reqCPU == 0) || (usageMemPercentage >= multiple || usageCPUPercentage >= multiple)
			if b {
				return &usageShowData{
					name:      pod.Name,
					namespace: pod.Namespace,

					requestMemory: req.Memory(),
					requestCPU:    req.Cpu(),

					limitMemory: limit.Memory(),
					limitCPU:    limit.Cpu(),

					usageMemory:           usage.Memory(),
					usageCPU:              usage.Cpu(),
					usageCPUPercentage:    usageCPUPercentageShow,
					usageMemroyPercentage: usageMemPercentageShow,
				}, nil
			}
		}
		return nil, nil
	}

	var lock sync.Mutex
	var g errgroup.Group

	var res []*usageShowData
	for _, pod := range pods {
		p := pod
		g.Go(func() error {
			if usage, err := filter(p); err != nil {
				return err
			} else if usage != nil {
				lock.Lock()
				res = append(res, usage)
				lock.Unlock()
			}
			return nil
		})
	}
	err = g.Wait()
	return res, err
}

func (l *UsageLister) Print(data []*usageShowData) error {
	sort.Slice(data, func(i, j int) bool {
		if data[i].usageCPUPercentage == "-" || data[i].usageMemroyPercentage == "-" {
			return false
		}
		if data[j].usageCPUPercentage == "-" || data[j].usageMemroyPercentage == "-" {
			return true
		}
		return data[i].name < data[j].name
	})

	l.writer.SetHeader([]string{"namespace", "name", "cpu(percentage)", "cpu(usage/request/limit)", "memory(percentage)", "memory(usage/request/limit)"})
	for _, val := range data {
		l.writer.Append(
			val.namespace, val.name,
			val.usageCPUPercentage,
			fmt.Sprintf("%dm / %dm / %dm", val.usageCPU.MilliValue(), val.requestCPU.MilliValue(), val.limitCPU.MilliValue()),
			val.usageMemroyPercentage,
			fmt.Sprintf("%dMi / %dMi / %dMi", val.usageMemory.Value()/(1024*1024), val.requestMemory.Value()/(1024*1024), val.limitMemory.Value()/(1024*1024)),
		)
	}
	return l.writer.Render()
}

// beautyUsage returns rawData and format percentage of (usage / request)%
func beautyUsage(a, b float64) (float64, string) {
	if b == 0 {
		return 0, "-"
	}
	res := a / b
	return res, fmt.Sprintf("%.1f%%", res*100)
}
