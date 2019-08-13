package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/reconquest/karma-go"
)

type container struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Root    string `json:"root"`
	Address string `json:"address"`
}

func queryContainers(
	args map[string]interface{}, storageEngine storage,
) error {
	var (
		rootDir = args["-r"].(string)
		useJSON = args["-j"].(bool)
		filter  = args["<name>"].([]string)
	)

	all, err := listContainers(filepath.Join(rootDir, "containers"))
	if err != nil {
		return err
	}

	active, err := listActiveContainers(containerSuffix)
	if err != nil {
		return err
	}

	containers := []container{}
	for _, name := range all {
		if len(filter) > 0 {
			found := false
			for _, target := range filter {
				if target == name {
					found = true
					break
				}
			}

			if !found {
				continue
			}
		}

		container := container{
			Name:    name,
			Status:  "inactive",
			Root:    storageEngine.GetContainerRoot(name),
			Address: "",
		}

		_, ok := active[name]
		if ok {
			container.Status = "active"
			container.Address, err = getContainerIP(name)
			if err != nil {
				fmt.Fprintln(os.Stderr, karma.Format(err,
					"WARNING: can't obtain container '%s' address",
					name,
				))
			}
		}

		containers = append(containers, container)
	}

	if !useJSON {
		writer := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		for _, container := range containers {
			fmt.Fprintf(
				writer,
				"%s\t%s\t%s\t%s\n",
				container.Name, container.Status,
				container.Address, container.Root,
			)
		}

		err = writer.Flush()
		if err != nil {
			return err
		}

		return nil
	}

	output, err := json.MarshalIndent(containers, "", "    ")
	if err != nil {
		return err
	}

	fmt.Println(string(output))

	return nil
}
