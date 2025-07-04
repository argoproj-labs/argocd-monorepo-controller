package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/triggers"
	"github.com/argoproj/notifications-engine/pkg/util/misc"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
	// "github.com/argoproj/argo-cd/v3/cmd/argocd/commands/admin"
)

func main() {
	command := &cobra.Command{
		Use: "gen",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	command.AddCommand(newDocsCommand())
	command.AddCommand(newCatalogCommand())

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newCatalogCommand() *cobra.Command {
	return &cobra.Command{
		Use: "catalog",
		Run: func(_ *cobra.Command, _ []string) {
			cm := corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "argocd-notifications-cm",
				},
				Data: make(map[string]string),
			}
			wd, err := os.Getwd()
			dieOnError(err, "Failed to get current working directory")
			target := path.Join(wd, "notifications_catalog/install.yaml")

			templatesDir := path.Join(wd, "notifications_catalog/templates")
			triggersDir := path.Join(wd, "notifications_catalog/triggers")

			templates, triggers, err := buildConfigFromFS(templatesDir, triggersDir)
			dieOnError(err, "Failed to build catalog config")

			misc.IterateStringKeyMap(triggers, func(name string) {
				trigger := triggers[name]
				t, err := yaml.Marshal(trigger)
				dieOnError(err, "Failed to marshal trigger")
				cm.Data["trigger."+name] = string(t)
			})

			misc.IterateStringKeyMap(templates, func(name string) {
				template := templates[name]
				t, err := yaml.Marshal(template)
				dieOnError(err, "Failed to marshal template")
				cm.Data["template."+name] = string(t)
			})

			d, err := yaml.Marshal(cm)
			dieOnError(err, "Failed to marshal final configmap")

			err = os.WriteFile(target, d, 0o644)
			dieOnError(err, "Failed to write builtin configmap")
		},
	}
}

func newDocsCommand() *cobra.Command {
	return &cobra.Command{
		Use: "docs",
		Run: func(_ *cobra.Command, _ []string) {
			var builtItDocsData bytes.Buffer
			wd, err := os.Getwd()
			dieOnError(err, "Failed to get current working directory")

			templatesDir := path.Join(wd, "notifications_catalog/templates")
			triggersDir := path.Join(wd, "notifications_catalog/triggers")

			notificationTemplates, notificationTriggers, err := buildConfigFromFS(templatesDir, triggersDir)
			dieOnError(err, "Failed to build builtin config")
			generateBuiltInTriggersDocs(&builtItDocsData, notificationTriggers, notificationTemplates)
			if err := os.WriteFile("./docs/operator-manual/notifications/catalog.md", builtItDocsData.Bytes(), 0o644); err != nil {
				log.Fatal(err)
			}
			var commandDocs bytes.Buffer
			if err := generateCommandsDocs(&commandDocs); err != nil {
				log.Fatal(err)
			}
			if err := os.WriteFile("./docs/operator-manual/notifications/troubleshooting-commands.md", commandDocs.Bytes(), 0o644); err != nil {
				log.Fatal(err)
			}
		},
	}
}

func generateBuiltInTriggersDocs(out io.Writer, triggers map[string][]triggers.Condition, templates map[string]services.Notification) {
	_, _ = fmt.Fprintln(out, "# Triggers and Templates Catalog")

	_, _ = fmt.Fprintln(out, "## Getting Started")
	_, _ = fmt.Fprintln(out, "* Install Triggers and Templates from the catalog")
	_, _ = fmt.Fprintln(out, "  ```bash")
	_, _ = fmt.Fprintln(out, "  kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/notifications_catalog/install.yaml")
	_, _ = fmt.Fprintln(out, "  ```")

	_, _ = fmt.Fprintln(out, "## Triggers")

	r := tw.Rendition{
		Borders: tw.Border{Left: tw.On, Top: tw.Off, Right: tw.On, Bottom: tw.Off},
		Symbols: tw.NewSymbolCustom("pipe-center").WithCenter("|").WithMidLeft("|").WithMidRight("|"),
	}
	c := tablewriter.NewConfigBuilder().WithRowAutoWrap(tw.WrapNone).Build()
	table := tablewriter.NewTable(out, tablewriter.WithConfig(c), tablewriter.WithRenderer(renderer.NewBlueprint(r)))
	table.Header("NAME", "DESCRIPTION", "TEMPLATE")
	misc.IterateStringKeyMap(triggers, func(name string) {
		t := triggers[name]
		desc := ""
		template := ""
		if len(t) > 0 {
			desc = t[0].Description
			template = strings.Join(t[0].Send, ",")
		}
		err := table.Append([]string{name, desc, fmt.Sprintf("[%s](#%s)", template, template)})
		if err != nil {
			panic(err)
		}
	})
	err := table.Render()
	if err != nil {
		panic(err)
	}

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "## Templates")
	misc.IterateStringKeyMap(templates, func(name string) {
		t := templates[name]
		yamlData, err := yaml.Marshal(t)
		if err != nil {
			panic(err)
		}
		_, _ = fmt.Fprintf(out, "### %s\n**definition**:\n```yaml\n%s\n```\n", name, string(yamlData))
	})
}

func generateCommandsDocs(out io.Writer) error {
	// create parents so that CommandPath() is correctly resolved
	mainCmd := &cobra.Command{Use: "argocd"}
	adminCmd := &cobra.Command{Use: "admin"}
	// toolCmd := admin.NewNotificationsCommand()
	// adminCmd.AddCommand(toolCmd)
	mainCmd.AddCommand(adminCmd)
	for _, mainSubCommand := range mainCmd.Commands() {
		for _, adminSubCommand := range mainSubCommand.Commands() {
			for _, toolSubCommand := range adminSubCommand.Commands() {
				for _, c := range toolSubCommand.Commands() {
					var cmdDesc bytes.Buffer
					if err := doc.GenMarkdown(c, &cmdDesc); err != nil {
						return fmt.Errorf("error generating Markdown for command: %v : %w", c, err)
					}
					for _, line := range strings.Split(cmdDesc.String(), "\n") {
						if strings.HasPrefix(line, "### SEE ALSO") {
							break
						}
						_, _ = fmt.Fprintf(out, "%s\n", line)
					}
				}
			}
		}
	}
	return nil
}

func dieOnError(err error, msg string) {
	if err != nil {
		fmt.Printf("[ERROR] %s: %v", msg, err)
		os.Exit(1)
	}
}

func buildConfigFromFS(templatesDir string, triggersDir string) (map[string]services.Notification, map[string][]triggers.Condition, error) {
	templatesCfg := map[string]services.Notification{}
	err := filepath.Walk(templatesDir, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			return fmt.Errorf("error navigating the templates dirctory: %s : %w", templatesDir, e)
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("error reading the template file: %s : %w", p, err)
		}
		name := strings.Split(path.Base(p), ".")[0]
		var template services.Notification
		if err := yaml.Unmarshal(data, &template); err != nil {
			return fmt.Errorf("error unmarshaling the data from file: %s : %w", p, err)
		}
		templatesCfg[name] = template
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	triggersCfg := map[string][]triggers.Condition{}
	err = filepath.Walk(triggersDir, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			return fmt.Errorf("error navigating the triggers dirctory: %s : %w", triggersDir, e)
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("error reading the trigger file: %s : %w", p, err)
		}
		name := strings.Split(path.Base(p), ".")[0]
		var trigger []triggers.Condition
		if err := yaml.Unmarshal(data, &trigger); err != nil {
			return fmt.Errorf("error unmarshaling the data from file: %s : %w", p, err)
		}
		triggersCfg[name] = trigger
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return templatesCfg, triggersCfg, nil
}
