# ansible-vnc-zsh

Builds two container images provisioned with
the <https://github.com/CowDogMoo/ansible-vnc-zsh> Ansible
playbook. One container runs with systemd and the other without.

## Build from Blueprint

From the root of the repo:

```bash
export OS="$(uname | python3 -c 'print(open(0).read().lower().strip())')"
cp "dist/warpgate_${OS}_arm64/wg" .
wg --config blueprints/ansible-vnc-zsh/config.yaml imageBuilder -p ~/cowdogmoo/ansible-vnc
```

## Run container locally

```bash
# Without systemd
docker run -dit --rm -p 5901:5901 cowdogmoo/ansible-vnc \
&& CONTAINER=$(docker ps | awk -F '  ' '{print $7}' | xargs) \
&& echo $CONTAINER && docker exec -it $CONTAINER zsh

# With systemd
docker run -d --privileged -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
  --rm -it -p 5901:5901 --cgroupns=host cowdogmoo/ansible-systemd-vnc \
&& CONTAINER=$(docker ps | awk -F '  ' '{print $7}' | xargs) \
&& echo $CONTAINER && docker exec -it $CONTAINER zsh
```

## Get vnc password

```bash
docker exec -it $CONTAINER zsh -c '/usr/local/bin/vncpwd /home/ubuntu/.vnc/passwd'
```

## Push image

Create a classic personal access token (fine-grained isn't supported yet)
with the following permissions taken from [here](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry):

- `read:packages`
- `write:packages`
- `delete:packages`

```bash
docker login ghcr.io -u USERNAME -p $PAT
docker push ghcr.io/cowdogmoo/ansible-vnc:latest
```

Built images can be found [here](https://github.com/orgs/CowDogMoo/packages).
