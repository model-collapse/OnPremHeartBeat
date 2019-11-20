FROM modelcollapse/raspbian-go:strech

RUN mkdir -p /tmp/workspace
RUN mkdir -p /tmp/grepo
ENV GOPATH /tmp/grepo
ENV GOPROXY goproxy.cn
WORKDIR /tmp/workspace
RUN mkdir OnPremHeartBeat
RUN cd OnPremHeartBeat & git clone https://github.com/model-collapse/OnPremHeartBeat.git . && go build -o OnPremHeartBeat
RUN chmod +x OnPremHeartBeat
RUN apt-get update
RUN apt-get install -y git

CMD ./OnPremHeartBeat --interval=10 --root_name="heartbeats" --zk_addr="n1.onprem.ai:2181"