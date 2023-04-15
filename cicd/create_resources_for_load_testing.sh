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
        "ssh_keys":[37976894,36058388,38048295],
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


retries=0
should_repeat=true
while $should_repeat; do
    ((retries+=1)) &&
    ssh -o "StrictHostKeyChecking=no" root@${PUBLIC_IPS[0]} 'whoami' &&
    should_repeat=false
    if (( $retries > 10 )); then
        echo $retries
        exit 1
    fi
    if $should_repeat; then
        sleep 5
    fi
done

ssh root@${PUBLIC_IPS[0]} -o "StrictHostKeyChecking=no" 'wget https://github.com/satur-io/estoraje/releases/latest/download/estoraje.tar.gz'
ssh root@${PUBLIC_IPS[0]} -o "StrictHostKeyChecking=no" 'tar -xf estoraje.tar.gz'
ssh root@${PUBLIC_IPS[0]} -o "StrictHostKeyChecking=no" \
    "nohup ./estoraje -name=node_1 \
    -initialCluster=node_1=http://${PRIVATE_IPS[0]}:2380,node_2=http://${PRIVATE_IPS[1]}:2380,node_3=http://${PRIVATE_IPS[2]}:2380 \
	-host=${PRIVATE_IPS[0]} \
	-port=8001 \
	-dataPath=data \
    > foo.log 2> foo.err < /dev/null &"


ssh root@${PUBLIC_IPS[1]} -o "StrictHostKeyChecking=no" 'wget https://github.com/satur-io/estoraje/releases/latest/download/estoraje.tar.gz'
ssh root@${PUBLIC_IPS[1]} -o "StrictHostKeyChecking=no" 'tar -xf estoraje.tar.gz'
ssh root@${PUBLIC_IPS[1]} -o "StrictHostKeyChecking=no" \
    "nohup ./estoraje -name=node_2 \
    -initialCluster=node_1=http://${PRIVATE_IPS[0]}:2380,node_2=http://${PRIVATE_IPS[1]}:2380,node_3=http://${PRIVATE_IPS[2]}:2380 \
	-host=${PRIVATE_IPS[1]} \
	-port=8001 \
	-dataPath=data \
    > foo.log 2> foo.err < /dev/null &"

ssh root@${PUBLIC_IPS[2]} -o "StrictHostKeyChecking=no" 'wget https://github.com/satur-io/estoraje/releases/latest/download/estoraje.tar.gz'
ssh root@${PUBLIC_IPS[2]} -o "StrictHostKeyChecking=no" 'tar -xf estoraje.tar.gz'
ssh root@${PUBLIC_IPS[2]} -o "StrictHostKeyChecking=no" \
    "nohup ./estoraje -name=node_3 \
    -initialCluster=node_1=http://${PRIVATE_IPS[0]}:2380,node_2=http://${PRIVATE_IPS[1]}:2380,node_3=http://${PRIVATE_IPS[2]}:2380 \
	-host=${PRIVATE_IPS[2]} \
	-port=8001 \
	-dataPath=data \
    > foo.log 2> foo.err < /dev/null &"

curl -X POST -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer '$DIGITAL_OCEAN_TOKEN'' \
    -d '{"name":"ubuntu-s-1vcpu-512mb-10gb-fra1-01",
        "size":"s-1vcpu-512mb-10gb",
        "region":"fra1",
        "image":"ubuntu-22-10-x64",
        "vpc_uuid":"566b9e0d-3f17-4832-908e-2d5ff23401c1",
        "ssh_keys":[37976894,36058388,38048295],
        "tags":["load-testing","test-load-balancer"]}' \
    "https://api.digitalocean.com/v2/droplets"


while [[ ${LOAD_BALANCER_PUBLIC_IP} = "" ]]; do
  sleep 5
  LOAD_BALANCER_PUBLIC_IP=($(curl -X GET \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DIGITAL_OCEAN_TOKEN" \
  "https://api.digitalocean.com/v2/droplets?tag_name=test-load-balancer" \
  | jq --raw-output '.droplets[].networks.v4[] | select(.type == "public") | .ip_address'))
done

echo ${LOAD_BALANCER_PUBLIC_IP}

retries=0
should_repeat=true
while $should_repeat; do
    ((retries+=1)) &&
    ssh -o "StrictHostKeyChecking=no" root@${LOAD_BALANCER_PUBLIC_IP} 'whoami' &&
    should_repeat=false
    if (( $retries > 10 )); then
        exit 1
    fi
    if $should_repeat; then
        sleep 5
    fi
done

ssh -o "StrictHostKeyChecking=no" root@${LOAD_BALANCER_PUBLIC_IP} 'snap install --edge caddy'
ssh -o "StrictHostKeyChecking=no" -fn root@${LOAD_BALANCER_PUBLIC_IP} \
    "cat <<EOF > /etc/systemd/system/caddy-lb.service
[Unit]
Description=Caddy load balancer
After=network.target
StartLimitIntervalSec=0
[Service]
Type=simple
Restart=always
RestartSec=1
ExecStart=caddy reverse-proxy --from ${LOAD_BALANCER_PUBLIC_IP} --to ${PRIVATE_IPS[0]}:8001 --to ${PRIVATE_IPS[1]}:8001 --to ${PRIVATE_IPS[2]}:8001
EOF"

ssh -o "StrictHostKeyChecking=no" -fn root@${LOAD_BALANCER_PUBLIC_IP} \ 'systemctl start caddy-lb'
 

sleep 5

curl --insecure -X POST -d "cluster-running" https://${LOAD_BALANCER_PUBLIC_IP}/test
curl --insecure https://${LOAD_BALANCER_PUBLIC_IP}/test
