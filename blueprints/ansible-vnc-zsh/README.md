# ansible-vnc-zsh

Builds two container images provisioned with
the <https://github.com/CowDogMoo/ansible-vnc-zsh>
Ansible role. One container runs with systemd and the other without.

---

## Build from Blueprint

From the root of the repo:

```bash
wg imageBuilder \
  -b ansible-vnc-zsh \
  -p ~/cowdogmoo/ansible-vnc-zsh
```

---

## Run container locally

```bash
# Without systemd
docker run -dit --rm -p 5901:5901 ghcr.io/cowdogmoo/ansible-vnc \
&& CONTAINER=$(docker ps | awk -F '  ' '{print $7}' | xargs) \
&& echo $CONTAINER && docker exec -it $CONTAINER zsh

# With systemd
docker run -d --privileged -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
  --rm -it -p 5901:5901 --cgroupns=host ghcr.io/cowdogmoo/ansible-systemd-vnc \
&& CONTAINER=$(docker ps | awk -F '  ' '{print $7}' | xargs) \
&& echo $CONTAINER && docker exec -it $CONTAINER zsh
```

## Get vnc password

```bash
docker exec -it $CONTAINER zsh -c '/usr/local/bin/vncpwd /home/ubuntu/.vnc/passwd'
```
