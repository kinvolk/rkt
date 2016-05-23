# Jenkins configuration

Jenkins will be available at this address:

https://jenkins-rkt.prod.coreos.systems/

## Global GitHub configuration

- Go to https://jenkins-rkt.prod.coreos.systems/configure
- Find the GitHub section
- API URL: https://api.github.com
- Credentials: some text (user rktbot)
- In https://github.com/coreos/rkt/settings/hooks, add a secret for https://jenkins-rkt.prod.coreos.systems/github-webhook/

## Cloud provisioning (Amazon EC2)

- Name: coreos-dev us-west-1

### Fedora-22

- Description: `Fedora-Cloud-Base-22-20160218.x86_64-us-west-1-HVM-standard-0`
- AMI ID: ami-e291e082
- Instance Type: M4Large
- Remote user: fedora
- Labels: rkt-distro fedora-22
- Number of Executors (Advanced section): 1

Init script:
```
#!/bin/bash

# ---pretend-input-tty (with 3 dashes) is an undocumented feature of parted
sudo parted /dev/xvda ---pretend-input-tty resizepart 1 yes 100%
sudo resize2fs /dev/xvda1

sudo /usr/sbin/setenforce 0

sudo chmod 755 /home/fedora

sudo dnf -y -v update
sudo dnf -y -v groupinstall "Development Tools" "C Development Tools and Libraries"
sudo dnf -y -v install wget squashfs-tools patch glibc-static gnupg golang libacl-devel file openssl-devel bc
# systemd-container only available in newer versions of Fedora
sudo dnf builddep -y -v systemd

sudo dnf -y -v install systemd-container || true
sudo dnf -y -v install java || true

sudo groupadd rkt || true
sudo gpasswd -a fedora rkt || true
```

### Debian-8

- Description: `debian-jessie-amd64-hvm-2016-04-03-ebs`
- AMI ID: ami-45374b25
- Instance Type: M4Large
- Remote user: admin
- Labels: rkt-distro debian-8
- Number of Executors (Advanced section): 1

Init script:
```
#!/bin/bash

set -e
set -x

# cloudinit is messing with /etc/apt. Wait until it's done
while ! sudo systemctl is-active cloud-config.target ; do sleep 1 ; done

sudo groupadd rkt || true
sudo gpasswd -a admin rkt || true

echo 'deb http://cloudfront.debian.net/debian testing main' | sudo tee -a /etc/apt/sources.list.d/testing.list
echo 'deb-src http://cloudfront.debian.net/debian testing main' | sudo tee -a /etc/apt/sources.list.d/testing.list

sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes build-essential autoconf
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confnew" squashfs-tools patch gnupg golang git dbus libacl1-dev systemd-container libssl-dev
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confnew" default-jre
sudo DEBIAN_FRONTEND=noninteractive apt-get build-dep -y --force-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confnew" systemd
```

### Centos-7

- Description: `CentOS Linux 7 x86_64 HVM EBS 1602-b7ee8a69-ee97-4a49-9e68-afaee216db2e-ami-d7e1d2bd.3 by aws-marketplace`
- AMI ID: ami-af4333cf
- Instance Type: M4Large
- Remote user: centos
- Labels: rkt-distro centos-7
- Number of Executors (Advanced section): 1

Init script:
```
#!/bin/bash
set -e
set -x

sudo groupadd rkt || true
sudo gpasswd -a centos rkt || true

#yum update -y
sudo yum -y -v groupinstall "Development Tools"
sudo yum -y -v install wget squashfs-tools patch glibc-static gnupg libacl-devel openssl-devel bc
sudo yum -y -v install java # for Jenkins

pushd /tmp
wget -c https://storage.googleapis.com/golang/go1.5.3.linux-amd64.tar.gz
popd
sudo tar -C /usr/local -xzf /tmp/go1.5.3.linux-amd64.tar.gz
sudo ln -sf /usr/local/go/bin/go /usr/local/bin/go
sudo ln -sf /usr/local/go/bin/godoc /usr/local/bin/godoc
sudo ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
export GOROOT=/usr/local/go

sudo /usr/sbin/setenforce 0

# sudo/su without tty is disabled on old CentOS.
# https://bugzilla.redhat.com/show_bug.cgi?id=1020147
# Cannot workaround with /usr/bin/expect because this would need expect > 2.27
sudo sed -i 's/ requiretty$/ !requiretty/g' /etc/sudoers
```

### Fedora-23

- Description: `Fedora-Cloud-Base-23-20160518.x86_64-us-west-1-HVM-standard-0`
- AMI ID: ami-0fbec66f
- Instance Type: M4Large
- Remote user: fedora
- Labels: rkt-distro fedora-23
- Number of Executors (Advanced section): 1

Init script:
```
#!/bin/bash

# ---pretend-input-tty (with 3 dashes) is an undocumented feature of parted
sudo parted /dev/xvda ---pretend-input-tty resizepart 1 yes 100%
sudo resize2fs /dev/xvda1

sudo /usr/sbin/setenforce 0

sudo chmod 755 /home/fedora

sudo dnf -y -v update
sudo dnf -y -v groupinstall "Development Tools" "C Development Tools and Libraries"
sudo dnf -y -v install wget squashfs-tools patch glibc-static gnupg golang libacl-devel file openssl-devel bc
# systemd-container only available in newer versions of Fedora
sudo dnf -y -v install systemd-container || true
sudo dnf -y -v install java || true
sudo dnf builddep -y -v systemd

sudo groupadd rkt || true
sudo gpasswd -a fedora rkt || true
```

### Fedora-24

- Description: `Fedora-Cloud-Base-24-20160516.n.0.x86_64-us-west-1-HVM-standard-0`
- AMI ID: ami-16542c76
- Instance Type: M4Large
- Remote user: fedora
- Labels: rkt-distro fedora-24
- Number of Executors (Advanced section): 1

Init script:
```
#!/bin/bash

# ---pretend-input-tty (with 3 dashes) is an undocumented feature of parted
sudo parted /dev/xvda ---pretend-input-tty resizepart 1 yes 100%
sudo resize2fs /dev/xvda1

sudo /usr/sbin/setenforce 0

sudo chmod 755 /home/fedora

sudo dnf -y -v update
sudo dnf -y -v groupinstall "Development Tools" "C Development Tools and Libraries"
sudo dnf -y -v install wget squashfs-tools patch glibc-static gnupg golang libacl-devel file openssl-devel bc
# systemd-container only available in newer versions of Fedora
sudo dnf -y -v install systemd-container || true
sudo dnf -y -v install java || true
sudo dnf builddep -y -v systemd

sudo groupadd rkt || true
sudo gpasswd -a fedora rkt || true
```

### Fedora-Rawhide

- Description: `Fedora-Cloud-Base-Rawhide-20160427.n.0.x86_64-us-west-1-HVM-standard-0`
- AMI ID: ami-bf4638df
- Instance Type: M4Large
- Remote user: fedora
- Labels: rkt-distro fedora-rawhide
- Number of Executors (Advanced section): 1

Init script:
```
#!/bin/bash

# ---pretend-input-tty (with 3 dashes) is an undocumented feature of parted
sudo parted /dev/xvda ---pretend-input-tty resizepart 1 yes 100%
sudo resize2fs /dev/xvda1

sudo /usr/sbin/setenforce 0

sudo chmod 755 /home/fedora

sudo dnf -y -v update
sudo dnf -y -v groupinstall "Development Tools" "C Development Tools and Libraries"
sudo dnf -y -v install wget squashfs-tools patch glibc-static gnupg golang libacl-devel file openssl-devel bc
# systemd-container only available in newer versions of Fedora
sudo dnf -y -v install systemd-container || true
sudo dnf -y -v install java || true
sudo dnf builddep -y -v systemd

sudo groupadd rkt || true
sudo gpasswd -a fedora rkt || true
```

### Fedora-Rawhide-Selinux

- Description: `Fedora-Cloud-Base-Rawhide-20160427.n.0.x86_64-us-west-1-HVM-standard-0`
- AMI ID: ami-bf4638df
- Instance Type: M4Large
- Remote user: fedora
- Labels: rkt-distro fedora-rawhide-selinux
- Number of Executors (Advanced section): 1

Init script:
```
#!/bin/bash

# ---pretend-input-tty (with 3 dashes) is an undocumented feature of parted
sudo parted /dev/xvda ---pretend-input-tty resizepart 1 yes 100%
sudo resize2fs /dev/xvda1

sudo chmod 755 /home/fedora

sudo dnf -y -v update
sudo dnf -y -v groupinstall "Development Tools" "C Development Tools and Libraries"
sudo dnf -y -v install wget squashfs-tools patch glibc-static gnupg golang libacl-devel file openssl-devel bc
# systemd-container only available in newer versions of Fedora
sudo dnf -y -v install systemd-container || true
sudo dnf -y -v install java || true
sudo dnf builddep -y -v systemd

sudo groupadd rkt || true
sudo gpasswd -a fedora rkt || true
```
