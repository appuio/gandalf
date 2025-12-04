Given I have all prerequisites installed
And I have the `openshift-install` binary for version "4.19"
And a lieutenant cluster
And Cloudscale API tokens
And a personal VSHN GitLab access token
And a control.vshn.net Servers API token
And basic cluster information
Then I download the OpenShift image for version "4.19.10"
And I set up required S3 buckets
And I import the image in Cloudscale
Then I set secrets in Vault
And I check the cluster domain
And I prepare the cluster repository
Then I configure the OpenShift installer
And I configure Terraform for team "aldebaran"
Then I provision the loadbalancers
And I provision the bootstrap node
