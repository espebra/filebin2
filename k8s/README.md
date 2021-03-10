**Filebin2 deploying to Kubernetes**

## Building the filebin2 binary and creating a docker image.

The filebin2 binary is best created by a CI/CD pipeline the binary should be built with static links using the following command.

go build -tags netgo -ldflags '-extldflags "-static"'

The binary can then be copied to the docker subdirectory and the docker container can be built using the Dockerfile in the docker sub directory. The resulting docker image needs to be pushed to dockerhub.

## Build your database
Filebin2 requires a Postgres database to store state for all the K8S deployments of the binary. Build a Postgres database and set it up using the SQL commands in  the schema.sql file.
 Ensure you know the database DNS port number and the username and password for the database as you will need that to configure the k8s deployments.


## Deploying to Kubernetes

Only two yaml files are required for deployment to k8s one is a deployment and the second a service. To install them you need access to a k8s cluster and the kubectl command.

first create a namespace called filebin2

**kubectl create namespace filebin2**

Then edit the deployment and service files with your details and deploy them. The service file also shows the annotations needed for an SSL load balancer using AWS, for this create an SSL certificate first.

Then deploy.

**kubectl -n filebin2 apply -f  deployments/filebin2-deployment.yaml**

and

**kubectl -n filebin2 apply -f  service/filebin2-service.yaml**

At this point you should have a working version of filebin2 running on k8s congrats!


> Written with [StackEdit](https://stackedit.io/).
