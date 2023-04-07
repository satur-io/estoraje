curl -X POST -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer '$DIGITAL_OCEAN_TOKEN'' \
    -d '{"names":["ubuntu-s-1vcpu-1gb-intel-fra1-01",
        "ubuntu-s-1vcpu-1gb-intel-fra1-02",
        "ubuntu-s-1vcpu-1gb-intel-fra1-03"],
        "size":"s-1vcpu-1gb-intel",
        "region":"fra1",
        "image":"ubuntu-22-10-x64",
        "monitoring":true,
        "vpc_uuid":"566b9e0d-3f17-4832-908e-2d5ff23401c1",
        "ssh_keys":[37976894,36058388],
        "tags":["load-testing"]}' \
    "https://api.digitalocean.com/v2/droplets"

declare PRIVATE_IPS=()
while [[ ${#PRIVATE_IPS[@]} < 3 ]]; do
  sleep 5
  PRIVATE_IPS=($(curl -X GET \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DIGITAL_OCEAN_TOKEN" \
  "https://api.digitalocean.com/v2/droplets?tag_name=load-testing" \
  | jq --raw-output '.droplets[].networks.v4[] | select(.type == "private") | .ip_address'))
done

echo "${#PRIVATE_IPS[@]} private ips"
echo $PRIVATE_IPS

declare PUBLIC_IPS=()
while [[ ${#PUBLIC_IPS[@]} < 3 ]]; do
  sleep 5
  PUBLIC_IPS=($(curl -X GET \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DIGITAL_OCEAN_TOKEN" \
  "https://api.digitalocean.com/v2/droplets?tag_name=load-testing" \
  | jq --raw-output '.droplets[].networks.v4[] | select(.type == "public") | .ip_address'))
done

echo "${#PUBLIC_IPS[@]} public ips"
echo $PUBLIC_IPS

ssh root@${PUBLIC_IPS[0]} 'wget https://github.com/satur-io/estoraje/releases/download/v0.0.2/estoraje-v0.0.2-linux-amd64.tar.gz'
ssh root@${PUBLIC_IPS[0]} 'tar -xf estoraje-v0.0.2-linux-amd64.tar.gz'
ssh root@${PUBLIC_IPS[0]} './estoraje -name=node_1 -initialCluster=node_1=https://10.114.0.2:2380,node_2=https://10.114.0.3:2380,node_3=https://10.114.0.4:2380 \
	-host=10.114.0.2 \
	-port=8001 \
	-dataPath=data &'




# curl -X POST -H 'Content-Type: application/json' \
#     -H 'Authorization: Bearer '$DIGITAL_OCEAN_TOKEN'' \
#     -d '{"name":"ubuntu-s-1vcpu-512mb-10gb-fra1-01",
#         "size":"s-1vcpu-512mb-10gb",
#         "region":"fra1",
#         "image":"ubuntu-22-10-x64",
#         "vpc_uuid":"566b9e0d-3f17-4832-908e-2d5ff23401c1",
#         "ssh-keys":[37976894,36058388]
#         "tags":["load-testing"]}' \
#     "https://api.digitalocean.com/v2/droplets"
