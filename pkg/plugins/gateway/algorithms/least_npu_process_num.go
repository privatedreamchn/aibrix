/*
Copyright 2024 The Aibrix Team.

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

package routingalgorithms

import (
	"math"
	"math/rand"

	"github.com/vllm-project/aibrix/pkg/cache"
	"github.com/vllm-project/aibrix/pkg/metrics"
	"github.com/vllm-project/aibrix/pkg/types"
	"github.com/vllm-project/aibrix/pkg/utils"
	v1 "k8s.io/api/core/v1"

	"k8s.io/klog/v2"
)

const RouterLeastProcess types.RoutingAlgorithm = "least-process"

func init() {
	Register(RouterLeastProcess, NewLeastProcessRouter)
}

type leastProcessRouter struct {
	cache cache.Cache
}

func NewLeastProcessRouter() (types.Router, error) {
	c, err := cache.Get()
	if err != nil {
		return nil, err
	}

	return leastProcessRouter{
		cache: c,
	}, nil
}

// Route process based of least npu process among input ready pods
func (r leastProcessRouter) Route(ctx *types.RoutingContext, readyPodList types.PodList) (string, error) {
	readyPods := readyPodList.All()
	targetPod := selectTargetPodWithLeastProcessCount(r.cache, readyPods)

	// Use fallback if no valid metrics
	if targetPod == nil {
		var err error
		targetPod, err = SelectRandomPodAsFallback(ctx, readyPods, rand.Intn)
		if err != nil {
			return "", err
		}
	}

	ctx.SetTargetPod(targetPod)
	return ctx.TargetAddress(), nil
}

func (r *leastProcessRouter) SubscribedMetrics() []string {
	return []string{
		metrics.NpuChipInfoProcessInfoNumber,
	}
}

func selectTargetPodWithLeastProcessCount(cache cache.Cache, readyPods []*v1.Pod) *v1.Pod {
	var targetPod *v1.Pod
	targetPods := []string{}

	minCount := math.MaxInt32
	podProcessCount := getProcessCounts(cache, readyPods)
	klog.V(4).InfoS("selectTargetPodWithLeastProcessCount", "podProcessCount", podProcessCount)
	for podname, podProcess := range podProcessCount {
		if podProcess < minCount {
			minCount = podProcess
			targetPods = []string{podname}
		} else if podProcess == minCount {
			targetPods = append(targetPods, podname)
		}
	}
	if len(targetPods) > 0 {
		targetPod, _ = utils.FilterPodByName(targetPods[rand.Intn(len(targetPods))], readyPods)
	}
	return targetPod
}

// getProcessCounts returns running npu chip avg process count for each pod tracked by gateway.
func getProcessCounts(cache cache.Cache, readyPods []*v1.Pod) map[string]int {
	podProcesstCount := map[string]int{}
	for _, pod := range readyPods {
		runningReq, err := cache.GetMetricValueByPod(pod.Name, pod.Namespace, metrics.NpuChipInfoProcessInfoNumber)
		if err != nil {
			runningReq = &metrics.SimpleMetricValue{Value: 0}
		}
		podProcesstCount[pod.Name] = int(runningReq.GetSimpleValue())
	}

	return podProcesstCount
}
