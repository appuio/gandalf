Given I have all prerequisites installed
And I have the `openshift-install` binary for version "4.20"
And a lieutenant cluster
And a Keycloak service
And Cloudscale API tokens
And a personal VSHN GitLab access token
And a control.vshn.net Servers API token
And basic cluster information
Then I download the OpenShift image for version "4.20.0"
And I set up required S3 buckets
And I import the image in Cloudscale
Then I set secrets in Vault
And I check the cluster domain
And I prepare the cluster repository
Then I configure the OpenShift installer
And I configure Terraform for team "aldebaran"
Then I provision the loadbalancers
And I provision the bootstrap node
And I store the subnet ID and floating IP in the Syn hierarchy
And I provision the control plane
Then I deploy initial manifests
And I wait for bootstrap to complete
Then I remove the bootstrap node
And I configure initial deployments
And I wait for installation to complete
Then I synthesize the cluster
Then I set acme-dns CNAME records
And I verify emergency access
And I configure the cluster alerts
And I enable Opsgenie alerting
And I verify the image registry config
And I schedule the first maintenance
Then I configure apt-dater groups for the LoadBalancers
And I remove the bootstrap bucket
And I add the cluster to openshift4-clusters
And I wait for maintenance to complete
