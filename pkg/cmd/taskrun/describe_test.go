// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package taskrun

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	tb "github.com/tektoncd/pipeline/test/builder"
	pipelinetest "github.com/tektoncd/pipeline/test/v1alpha1"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var (
	reasonCompleted = corev1.ContainerStateTerminated{Reason: "Completed"}
	reasonWaiting   = corev1.ContainerStateWaiting{Reason: "PodInitializing"}
	reasonFailed    = corev1.ContainerStateTerminated{Reason: "Error"}
	reasonRunning   = corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}
)

func TestTaskRunDescribe_invalid_namespace(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
		},
	})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	taskrun := Command(p)
	_, err := test.ExecuteCommand(taskrun, "desc", "bar", "-n", "invalid")
	if err == nil {
		t.Errorf("Expected error but did not get one")
	}
	expected := "namespaces \"invalid\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestTaskRunDescribe_not_found(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	taskrun := Command(p)
	_, err := test.ExecuteCommand(taskrun, "desc", "bar", "-n", "ns")
	if err == nil {
		t.Errorf("Expected error but did not get one")
	}
	expected := "failed to find taskrun \"bar\""
	test.AssertOutput(t, expected, err.Error())
}

func TestTaskRunDescribe_empty_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunLabel("tekton.dev/task", "t1"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("t1")),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_only_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clockwork.NewFakeClock().Now().Add(20*time.Second)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName("step1"),
					tb.SetStepStateTerminated(reasonCompleted),
				),
				tb.StepState(
					cb.StepName("step2"),
					tb.SetStepStateTerminated(reasonCompleted),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input", "param")),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input2", "param2")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("git", tb.TaskResourceBindingRef("git"))),
				tb.TaskRunInputs(tb.TaskRunInputsResource("image-input", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output2", tb.TaskResourceBindingRef("image"))),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_failed(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Status:  corev1.ConditionFalse,
					Reason:  resources.ReasonFailed,
					Message: "Testing tr failed",
				}),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_no_taskref(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Status:  corev1.ConditionFalse,
					Reason:  resources.ReasonFailed,
					Message: "Testing tr failed",
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_no_resourceref(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clockwork.NewFakeClock().Now().Add(20*time.Second)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName("step1"),
					tb.SetStepStateTerminated(reasonCompleted),
				),
				tb.StepState(
					cb.StepName("step2"),
					tb.SetStepStateTerminated(reasonCompleted),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input", "param")),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input2", "param2")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("git")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("image-input", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output")),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output2")),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_step_sidecar_status_defaults_and_failures(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clockwork.NewFakeClock().Now().Add(20*time.Second)),
				tb.StatusCondition(apis.Condition{
					Status: corev1.ConditionFalse,
					Reason: resources.ReasonFailed,
				}),
				tb.StepState(
					cb.StepName("step1"),
					tb.SetStepStateTerminated(reasonFailed),
				),
				tb.StepState(
					cb.StepName("step2"),
				),
				tb.SidecarState(
					tb.SidecarStateName("sidecar1"),
					tb.SetSidecarStateTerminated(reasonFailed),
				),
				tb.SidecarState(
					tb.SidecarStateName("sidecar2"),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input", "param")),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input2", "param2")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("git")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("image-input", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output")),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output2")),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_step_status_pending_one_sidecar(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clockwork.NewFakeClock().Now().Add(20*time.Second)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: "Running",
				}),
				tb.StepState(
					cb.StepName("step1"),
					tb.SetStepStateWaiting(reasonWaiting),
				),
				tb.StepState(
					cb.StepName("step2"),
					tb.SetStepStateWaiting(reasonWaiting),
				),
				tb.SidecarState(
					tb.SidecarStateName("sidecar1"),
					tb.SetSidecarStateWaiting(reasonWaiting),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input", "param")),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input2", "param2")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("git")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("image-input", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output")),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output2")),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_step_status_running_multiple_sidecars(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clockwork.NewFakeClock().Now().Add(20*time.Second)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: "Running",
				}),
				tb.StepState(
					cb.StepName("step1"),
					tb.SetStepStateRunning(reasonRunning),
				),
				tb.StepState(
					cb.StepName("step2"),
					tb.SetStepStateRunning(reasonRunning),
				),
				tb.SidecarState(
					tb.SidecarStateName("sidecar1"),
					tb.SetSidecarStateRunning(reasonRunning),
				),
				tb.SidecarState(
					tb.SidecarStateName("sidecar2"),
					tb.SetSidecarStateRunning(reasonRunning),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("t1"),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input", "param")),
				tb.TaskRunInputs(tb.TaskRunInputsParam("input2", "param2")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("git")),
				tb.TaskRunInputs(tb.TaskRunInputsResource("image-input", tb.TaskResourceBindingRef("image"))),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output")),
				tb.TaskRunOutputs(tb.TaskRunOutputsResource("image-output2")),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestTaskRunDescribe_cancel_taskrun(t *testing.T) {
	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-1", "ns",
			tb.TaskRunStatus(
				tb.TaskRunStartTime(clock.Now().Add(2*time.Minute)),
				cb.TaskRunCompletionTime(clock.Now().Add(5*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Status:  corev1.ConditionFalse,
					Reason:  "TaskRunCancelled",
					Message: "TaskRun \"tr-1\" was cancelled",
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	taskrun := Command(p)
	clock.Advance(10 * time.Minute)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-1", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}

func TestPipelineRunsDescribe_custom_output(t *testing.T) {
	name := "task-run"
	expected := "taskrun.tekton.dev/" + name

	clock := clockwork.NewFakeClock()

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(name, "ns"),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}
	pipelinerun := Command(p)

	got, err := test.ExecuteCommand(pipelinerun, "desc", "-o", "name", "-n", "ns", name)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	got = strings.TrimSpace(got)
	if got != expected {
		t.Errorf("Result should be '%s' != '%s'", got, expected)
	}
}

func TestTaskRunDescribe_custom_timeout(t *testing.T) {
	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr-custom-timeout", "ns",
			tb.TaskRunSpec(tb.TaskRunTimeout(time.Minute)),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		TaskRuns: trs,
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
				},
			},
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	taskrun := Command(p)
	actual, err := test.ExecuteCommand(taskrun, "desc", "tr-custom-timeout", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	golden.Assert(t, actual, fmt.Sprintf("%s.golden", t.Name()))
}
