package e2e

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/oam-dev/kubevela/pkg/server/apis"
	"github.com/oam-dev/kubevela/pkg/server/util"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

var (
	// SystemInitContext used for test install
	SystemInitContext = func(context string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("Install OAM runtime and vela builtin capabilities.", func() {
				output, err := Exec("vela install")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("- Installing OAM Kubernetes Runtime"))
				gomega.Expect(output).To(gomega.ContainSubstring("- Installing builtin capabilities"))
				gomega.Expect(output).To(gomega.ContainSubstring("Successful applied"))
			})
		})
	}

	SystemUpdateContext = func(context string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("Synchronize workload/trait definitions from cluster", func() {
				output, err := Exec("vela system update")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("workload definitions successfully synced"))
				gomega.Expect(output).To(gomega.ContainSubstring("trait definitions successfully synced"))
			})
		})
	}

	// RefreshContext used for test vela system update
	RefreshContext = func(context string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("Sync commands from your Kubernetes cluster and locally cached them", func() {
				output, err := Exec("vela system update")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("syncing workload definitions from cluster..."))
				gomega.Expect(output).To(gomega.ContainSubstring("sync"))
				gomega.Expect(output).To(gomega.ContainSubstring("successfully"))
				gomega.Expect(output).To(gomega.ContainSubstring("remove"))
			})
		})
	}

	// EnvInitContext used for test Env
	EnvInitContext = func(context string, envName string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should print env initiation successful message", func() {
				cli := fmt.Sprintf("vela env init %s --namespace %s", envName, envName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				expectedOutput := fmt.Sprintf("ENV %s CREATED,", envName)
				gomega.Expect(output).To(gomega.ContainSubstring(expectedOutput))
			})
		})
	}

	EnvShowContext = func(context string, envName string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should show detailed env message", func() {
				cli := fmt.Sprintf("vela env ls %s", envName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("NAME"))
				gomega.Expect(output).To(gomega.ContainSubstring("NAMESPACE"))
				gomega.Expect(output).To(gomega.ContainSubstring(envName))
			})
		})
	}

	EnvSetContext = func(context string, envName string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should show env set message", func() {
				cli := fmt.Sprintf("vela env sw %s", envName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				expectedOutput := fmt.Sprintf("Set env succeed, current env is %s", envName)
				gomega.Expect(output).To(gomega.ContainSubstring(expectedOutput))
			})
		})
	}

	EnvDeleteContext = func(context string, envName string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should delete an env", func() {
				cli := fmt.Sprintf("vela env delete %s", envName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				expectedOutput := fmt.Sprintf("%s deleted", envName)
				gomega.Expect(output).To(gomega.ContainSubstring(expectedOutput))
			})
		})
	}
	EnvDeleteCurrentUsingContext = func(context string, envName string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should delete all envs", func() {
				cli := fmt.Sprintf("vela env delete %s", envName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				expectedOutput := fmt.Sprintf("Error: you can't delete current using env %s", envName)
				gomega.Expect(output).To(gomega.ContainSubstring(expectedOutput))
			})
		})
	}

	//WorkloadRunContext used for test vela comp run
	WorkloadRunContext = func(context string, cli string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should print successful creation information", func() {
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("SUCCEED"))
			})
		})
	}

	WorkloadDeleteContext = func(context string, applicationName string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should print successful deletion information", func() {
				cli := fmt.Sprintf("vela app delete %s", applicationName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("delete apps succeed"))
			})
		})
	}

	// TraitManualScalerAttachContext used for test trait attach success
	TraitManualScalerAttachContext = func(context string, traitAlias string, applicationName string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should print successful attached information", func() {
				cli := fmt.Sprintf("vela %s %s", traitAlias, applicationName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("Adding " + traitAlias + " for app"))
				gomega.Expect(output).To(gomega.ContainSubstring("Succeeded!"))
			})
		})
	}

	// ComponentListContext used for test vela comp ls
	ComponentListContext = func(context string, applicationName string, traitAlias string) bool {
		return ginkgo.Context("ls", func() {
			ginkgo.It("should list all applications", func() {
				output, err := Exec("vela comp ls")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring("NAME"))
				gomega.Expect(output).To(gomega.ContainSubstring(applicationName))
				if traitAlias != "" {
					gomega.Expect(output).To(gomega.ContainSubstring(traitAlias))
				}
			})
		})
	}

	ApplicationStatusContext = func(context string, applicationName string, workloadType string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should get status for the application", func() {
				cli := fmt.Sprintf("vela app status %s", applicationName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring(applicationName))
				// TODO(roywang) add more assertion to check health status
			})
		})
	}

	ApplicationCompStatusContext = func(context string, applicationName string, workloadType string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should get status for the component", func() {
				cli := fmt.Sprintf("vela comp status %s", applicationName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(output).To(gomega.ContainSubstring(applicationName))
				// TODO(zzxwill) need to check workloadType after app status is refined
			})
		})
	}

	ApplicationShowContext = func(context string, applicationName string, workloadType string) bool {
		return ginkgo.Context(context, func() {
			ginkgo.It("should show app information", func() {
				cli := fmt.Sprintf("vela app show %s", applicationName)
				output, err := Exec(cli)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// TODO(zzxwill) need to check workloadType after app show is refined
				//gomega.Expect(output).To(gomega.ContainSubstring(workloadType))
				gomega.Expect(output).To(gomega.ContainSubstring(applicationName))
			})
		})
	}

	// APIEnvInitContext used for test api env
	APIEnvInitContext = func(context string, envMeta apis.Environment) bool {
		return ginkgo.Context("Post /envs/", func() {
			ginkgo.It("should create an env", func() {
				data, err := json.Marshal(&envMeta)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				resp, err := http.Post(util.URL("/envs/"), "application/json", strings.NewReader(string(data)))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer resp.Body.Close()
				result, err := ioutil.ReadAll(resp.Body)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				var r apis.Response
				err = json.Unmarshal(result, &r)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(http.StatusOK).Should(gomega.Equal(r.Code))
				gomega.Expect(r.Data.(string)).To(gomega.ContainSubstring("CREATED"))
			})
		})
	}
)
