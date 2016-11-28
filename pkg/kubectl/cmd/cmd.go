/*
Copyright 2014 The Kubernetes Authors.

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

package cmd

import (
	"fmt"
	"io"
	"os"

	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/auth"
	cmdconfig "k8s.io/kubernetes/pkg/kubectl/cmd/config"
	"k8s.io/kubernetes/pkg/kubectl/cmd/rollout"
	"k8s.io/kubernetes/pkg/kubectl/cmd/set"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/plugins"
	"k8s.io/kubernetes/pkg/util/i18n"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	bash_completion_func = `# call kubectl get $1,
__kubectl_override_flag_list=(kubeconfig cluster user context namespace server)
__kubectl_override_flags()
{
    local ${__kubectl_override_flag_list[*]} two_word_of of
    for w in "${words[@]}"; do
        if [ -n "${two_word_of}" ]; then
            eval "${two_word_of}=\"--${two_word_of}=\${w}\""
            two_word_of=
            continue
        fi
        for of in "${__kubectl_override_flag_list[@]}"; do
            case "${w}" in
                --${of}=*)
                    eval "${of}=\"${w}\""
                    ;;
                --${of})
                    two_word_of="${of}"
                    ;;
            esac
        done
        if [ "${w}" == "--all-namespaces" ]; then
            namespace="--all-namespaces"
        fi
    done
    for of in "${__kubectl_override_flag_list[@]}"; do
        if eval "test -n \"\$${of}\""; then
            eval "echo \${${of}}"
        fi
    done
}

__kubectl_get_namespaces()
{
    local template kubectl_out
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"
    if kubectl_out=$(kubectl get -o template --template="${template}" namespace 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${kubectl_out[*]}" -- "$cur" ) )
    fi
}

__kubectl_get_contexts()
{
    local template kubectl_out
    template="{{ range .contexts  }}{{ .name }} {{ end }}"
    if kubectl_out=$(kubectl config $(__kubectl_override_flags) -o template --template="${template}" view 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${kubectl_out[*]}" -- "$cur" ) )
    fi
}

__kubectl_parse_get()
{
    local template
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"
    local kubectl_out
    if kubectl_out=$(kubectl get $(__kubectl_override_flags) -o template --template="${template}" "$1" 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${kubectl_out[*]}" -- "$cur" ) )
    fi
}

__kubectl_get_resource()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        return 1
    fi
    __kubectl_parse_get "${nouns[${#nouns[@]} -1]}"
}

__kubectl_get_resource_pod()
{
    __kubectl_parse_get "pod"
}

__kubectl_get_resource_rc()
{
    __kubectl_parse_get "rc"
}

__kubectl_get_resource_node()
{
    __kubectl_parse_get "node"
}

# $1 is the name of the pod we want to get the list of containers inside
__kubectl_get_containers()
{
    local template
    template="{{ range .spec.containers  }}{{ .name }} {{ end }}"
    __debug "${FUNCNAME} nouns are ${nouns[*]}"

    local len="${#nouns[@]}"
    if [[ ${len} -ne 1 ]]; then
        return
    fi
    local last=${nouns[${len} -1]}
    local kubectl_out
    if kubectl_out=$(kubectl get $(__kubectl_override_flags) -o template --template="${template}" pods "${last}" 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${kubectl_out[*]}" -- "$cur" ) )
    fi
}

# Require both a pod and a container to be specified
__kubectl_require_pod_and_container()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        __kubectl_parse_get pods
        return 0
    fi;
    __kubectl_get_containers
    return 0
}

__custom_func() {
    case ${last_command} in
        kubectl_get | kubectl_describe | kubectl_delete | kubectl_label | kubectl_stop | kubectl_edit | kubectl_patch |\
        kubectl_annotate | kubectl_expose | kubectl_scale | kubectl_autoscale | kubectl_taint | kubectl_rollout_*)
            __kubectl_get_resource
            return
            ;;
        kubectl_logs | kubectl_attach)
            __kubectl_require_pod_and_container
            return
            ;;
        kubectl_exec | kubectl_port-forward | kubectl_top_pod)
            __kubectl_get_resource_pod
            return
            ;;
        kubectl_rolling-update)
            __kubectl_get_resource_rc
            return
            ;;
        kubectl_cordon | kubectl_uncordon | kubectl_drain | kubectl_top_node)
            __kubectl_get_resource_node
            return
            ;;
        *)
            ;;
    esac
}
`

	// If you add a resource to this list, please also take a look at pkg/kubectl/kubectl.go
	// and add a short forms entry in expandResourceShortcut() when appropriate.
	// TODO: This should be populated using the discovery information from apiserver.
	valid_resources = `Valid resource types include:

    * all
    * certificatesigningrequests (aka 'csr')
    * clusterrolebindings
    * clusterroles
    * clusters (valid only for federation apiservers)
    * componentstatuses (aka 'cs')
    * configmaps (aka 'cm')
    * cronjobs
    * daemonsets (aka 'ds')
    * deployments (aka 'deploy')
    * endpoints (aka 'ep')
    * events (aka 'ev')
    * horizontalpodautoscalers (aka 'hpa')
    * ingresses (aka 'ing')
    * jobs
    * limitranges (aka 'limits')
    * namespaces (aka 'ns')
    * networkpolicies
    * nodes (aka 'no')
    * persistentvolumeclaims (aka 'pvc')
    * persistentvolumes (aka 'pv')
    * poddisruptionbudgets (aka 'pdb')
    * pods (aka 'po')
    * podsecuritypolicies (aka 'psp')
    * podtemplates
    * replicasets (aka 'rs')
    * replicationcontrollers (aka 'rc')
    * resourcequotas (aka 'quota')
    * rolebindings
    * roles
    * secrets
    * serviceaccounts (aka 'sa')
    * services (aka 'svc')
    * statefulsets
    * storageclasses
    * thirdpartyresources
    `
)

// NewKubectlCommand creates the `kubectl` command and its nested children.
func NewKubectlCommand(f cmdutil.Factory, in io.Reader, out, errOut io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   "kubectl",
		Short: i18n.T("kubectl controls the Kubernetes cluster manager"),
		Long: templates.LongDesc(`
      kubectl controls the Kubernetes cluster manager.

      Find more information at https://github.com/kubernetes/kubernetes.`),
		Run: runHelp,
		BashCompletionFunction: bash_completion_func,
	}

	f.BindFlags(cmds.PersistentFlags())
	f.BindExternalFlags(cmds.PersistentFlags())

	// Sending in 'nil' for the getLanguageFn() results in using
	// the LANG environment variable.
	//
	// TODO: Consider adding a flag or file preference for setting
	// the language, instead of just loading from the LANG env. variable.
	i18n.LoadTranslations("kubectl", nil)

	// From this point and forward we get warnings on flags that contain "_" separators
	cmds.SetGlobalNormalizationFunc(flag.WarnWordSepNormalizeFunc)

	groups := templates.CommandGroups{
		{
			Message: "Basic Commands (Beginner):",
			Commands: []*cobra.Command{
				NewCmdCreate(f, out, errOut),
				NewCmdExposeService(f, out),
				NewCmdRun(f, in, out, errOut),
				set.NewCmdSet(f, out, errOut),
			},
		},
		{
			Message: "Basic Commands (Intermediate):",
			Commands: []*cobra.Command{
				NewCmdGet(f, out, errOut),
				NewCmdExplain(f, out, errOut),
				NewCmdEdit(f, out, errOut),
				NewCmdDelete(f, out, errOut),
			},
		},
		{
			Message: "Deploy Commands:",
			Commands: []*cobra.Command{
				rollout.NewCmdRollout(f, out, errOut),
				NewCmdRollingUpdate(f, out),
				NewCmdScale(f, out),
				NewCmdAutoscale(f, out),
			},
		},
		{
			Message: "Cluster Management Commands:",
			Commands: []*cobra.Command{
				NewCmdCertificate(f, out),
				NewCmdClusterInfo(f, out),
				NewCmdTop(f, out, errOut),
				NewCmdCordon(f, out),
				NewCmdUncordon(f, out),
				NewCmdDrain(f, out, errOut),
				NewCmdTaint(f, out),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				NewCmdDescribe(f, out, errOut),
				NewCmdLogs(f, out),
				NewCmdAttach(f, in, out, errOut),
				NewCmdExec(f, in, out, errOut),
				NewCmdPortForward(f, out, errOut),
				NewCmdProxy(f, out),
				NewCmdCp(f, in, out, errOut),
				auth.NewCmdAuth(f, out, errOut),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				NewCmdApply(f, out, errOut),
				NewCmdPatch(f, out),
				NewCmdReplace(f, out),
				NewCmdConvert(f, out),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				NewCmdLabel(f, out),
				NewCmdAnnotate(f, out),
				NewCmdCompletion(f, out, ""),
			},
		},
	}

	loadedPlugins, err := plugins.NewConfigDirPluginLoader().Load()
	if err != nil {
		fmt.Printf("Unable to load plugins due to: %v\n", err)
	}

	if len(loadedPlugins) > 0 {
		pluginCmds := []*cobra.Command{}

		for i := 0; i < len(loadedPlugins); i++ {
			plugin := loadedPlugins[i]

			pluginCmds = append(pluginCmds, &cobra.Command{
				Use:     plugin.Use,
				Short:   plugin.Short,
				Long:    templates.LongDesc(plugin.Long),
				Example: templates.Examples(plugin.Example),
				Run: func(cmd *cobra.Command, args []string) {
					env := os.Environ()

					if plugin.Tunnel {
						clientConfig, err := f.ClientConfig()
						if err != nil {
							glog.Fatal(err)
						}

						listener, err := plugins.ServePluginAPIProxy(clientConfig)
						if err != nil {
							glog.Fatal(err)
						}
						defer listener.Close()

						env = append(env, fmt.Sprintf("KUBECTL_PLUGIN_API_HOST=%s", listener.Addr()))
					}

					err := plugin.Run(in, out, errOut, env, args...)
					cmdutil.CheckErr(err)
				},
			})
		}

		groups = append(groups, templates.CommandGroup{
			Message:  "Plugins:",
			Commands: pluginCmds,
		})
	}

	groups.Add(cmds)

	filters := []string{
		"options",
		Deprecated("kubectl", "delete", cmds, NewCmdStop(f, out)),
	}
	templates.ActsAsRootCommand(cmds, filters, groups...)

	if cmds.Flag("namespace") != nil {
		if cmds.Flag("namespace").Annotations == nil {
			cmds.Flag("namespace").Annotations = map[string][]string{}
		}
		cmds.Flag("namespace").Annotations[cobra.BashCompCustom] = append(
			cmds.Flag("namespace").Annotations[cobra.BashCompCustom],
			"__kubectl_get_namespaces",
		)
	}

	if cmds.Flag("context") != nil {
		if cmds.Flag("context").Annotations == nil {
			cmds.Flag("context").Annotations = map[string][]string{}
		}
		cmds.Flag("context").Annotations[cobra.BashCompCustom] = append(
			cmds.Flag("context").Annotations[cobra.BashCompCustom],
			"__kubectl_get_contexts",
		)
	}

	cmds.AddCommand(cmdconfig.NewCmdConfig(clientcmd.NewDefaultPathOptions(), out, errOut))
	cmds.AddCommand(NewCmdVersion(f, out))
	cmds.AddCommand(NewCmdApiVersions(f, out))
	cmds.AddCommand(NewCmdOptions(out))

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func printDeprecationWarning(command, alias string) {
	glog.Warningf("%s is DEPRECATED and will be removed in a future version. Use %s instead.", alias, command)
}

func Deprecated(baseName, to string, parent, cmd *cobra.Command) string {
	cmd.Long = fmt.Sprintf("Deprecated: This command is deprecated, all its functionalities are covered by \"%s %s\"", baseName, to)
	cmd.Short = fmt.Sprintf("Deprecated: %s", to)
	parent.AddCommand(cmd)
	return cmd.Name()
}
