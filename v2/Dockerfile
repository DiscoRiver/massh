FROM ubuntu:latest
RUN apt update && apt install openssh-server sudo -y
RUN mkdir /var/run/sshd
RUN useradd -rm -d /home/ubuntu -s /bin/bash -g root -G sudo -u 1000 test
RUN  echo 'test:test' | chpasswd
RUN sed -i 's/PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
RUN sed 's@session\s*required\s*pam_loginuid.so@session optional pam_loginuid.so@g' -i /etc/pam.d/sshd
EXPOSE 22
CMD ["/usr/sbin/sshd","-D"]