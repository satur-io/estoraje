while [[ ${LOAD_BALANCER_PUBLIC_IP} = "" ]]; do
  sleep 5
  LOAD_BALANCER_PUBLIC_IP=($(curl -X GET \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DIGITAL_OCEAN_TOKEN" \
  "https://api.digitalocean.com/v2/droplets?tag_name=test-load-balancer" \
  | jq --raw-output '.droplets[].networks.v4[] | select(.type == "public") | .ip_address'))
done

pip install locust

locust -f test/locustfile.py --headless -u 100 -r 5 --run-time 5m -H https://${LOAD_BALANCER_PUBLIC_IP} --only-summary >> $GITHUB_STEP_SUMMARY

