# Files to stand up filebin2 in GKE

### Database setup

With a Cloud SQL database that's had a `filebin2` database added to it:

```
psql filebin2 -U postgres -h $SQL_IP -f schema.sql
```

### Extract from Google Cloud Shell

```
gcloud config set project filebin2
gcloud config set compute/region us-central1
gcloud container clusters get-credentials filebin2-cluster
mkdir filebin2-config
cd filebin2-config/
```

Copy in the modified contents of k8s/gke directory.

NB that BASE64 secrets are created with `echo -n 'secret' | base64`

The files should be applied in this order:

```
kubectl apply -f filebin2-secrets.yaml 
kubectl apply -f filebin2-deployment.yaml 
kubectl apply -f filebin2-service.yaml
kubectl apply -f filebin2-cert.yaml
kubectl apply -f filebin2-redirect.yaml
kubectl apply -f filebin2-ingress.yaml
```