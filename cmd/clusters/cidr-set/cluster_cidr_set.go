// Command listclusters lists all clusters and their node pools for a given project and zone.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2/google"

	container "google.golang.org/api/container/v1"
)

var (
	projectID = flag.String("project", "", "Project ID")
	zone      = flag.String("zone", "", "Compute zone")
	region    = flag.String("region", "", "Cluster region")
)

func main() {
	flag.Parse()

	if *projectID == "" {
		fmt.Fprintln(os.Stderr, "missing -project flag")
		flag.Usage()
		os.Exit(2)
	}
	if (*zone == "" && *region == "") || (*zone != "" && *region != "") {
		fmt.Fprintln(os.Stderr, "one of -zone or -region flag is required")
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()

	// See https://cloud.google.com/docs/authentication/.
	// Use GOOGLE_APPLICATION_CREDENTIALS environment variable to specify
	// a service account key file to authenticate to the API.
	hc, err := google.DefaultClient(ctx, container.CloudPlatformScope)
	if err != nil {
		log.Fatalf("Could not get authenticated client: %v", err)
	}

	svc, err := container.New(hc)
	if err != nil {
		log.Fatalf("Could not initialize gke client: %v", err)
	}

	if err := listClustersCIDR(svc, *projectID, *zone, *region); err != nil {
		log.Fatal(err)
	}
}

func listClustersCIDR(svc *container.Service, projectID, zone, region string) error {
	var (
		child string
	)
	switch {
	case zone != "":
		child = zone
	case region != "":
		child = region
	}

	ctx := context.Background()
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, child)
	list, err := svc.Projects.Locations.Clusters.List(parent).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to list zonal clusters: %v", err)
	}

	// DesiredMasterAuthorizedNetworksConfig *MasterAuthorizedNetworksConfig `json:"desiredMasterAuthorizedNetworksConfig,omitempty"`
	// MasterAuthorizedNetworksConfig *MasterAuthorizedNetworksConfig `json:"masterAuthorizedNetworksConfig,omitempty"`
	var cidr string
	for _, v := range list.Clusters {
		fmt.Printf("Cluster %q (%s) master_version: v%s\n", v.Name, v.Status, v.CurrentMasterVersion)
		if v.MasterAuthorizedNetworksConfig != nil && v.MasterAuthorizedNetworksConfig.Enabled {
			for _, cidrBlock := range v.MasterAuthorizedNetworksConfig.CidrBlocks {
				if cidrBlock == nil {
					continue
				}

				fmt.Printf("cidr: %s\n", cidrBlock.CidrBlock)
				cidr = cidrBlock.CidrBlock

				break // first one
			}

		}

		if cidr == "" {
			return errors.New("duh nothing found")
		}

		//func (r *ProjectsLocationsClustersService) Update(name string, updateclusterrequest *UpdateClusterRequest) *ProjectsLocationsClustersUpdateCall {
		/*
		 */
		cluster := fmt.Sprintf("%s/clusters/%s", parent, v.Name)
		resp, err := svc.Projects.Locations.Clusters.Update(cluster, &container.UpdateClusterRequest{
			Name: cluster,
			Update: &container.ClusterUpdate{
				DesiredMasterAuthorizedNetworksConfig: &container.MasterAuthorizedNetworksConfig{
					Enabled: true,
					CidrBlocks: []*container.CidrBlock{
						&container.CidrBlock{
							CidrBlock: cidr,
						},
					},
				},
			},
		}).Context(ctx).Do()

		if err != nil {
			return err
		}

		fmt.Printf("resp: %#v\n", resp)

		break // first one
		/*
			cluster := fmt.Sprintf("%s/clusters/%s", parent, v.Name)
			poolList, err := svc.Projects.Locations.Clusters.NodePools.List(cluster).Do()
			if err != nil {
				return fmt.Errorf("failed to list node pools for cluster %q: %v", v.Name, err)
			}
			for _, np := range poolList.NodePools {
				fmt.Printf("  -> Pool %q (%s) machineType=%s node_version=v%s autoscaling=%v\n", np.Name, np.Status,
					np.Config.MachineType, np.Version, np.Autoscaling != nil && np.Autoscaling.Enabled)
			}
		*/
	}
	return nil
}
