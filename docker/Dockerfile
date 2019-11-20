FROM modelcollapse/raspbian-go:strech

RUN mkdir -p /tmp/workspace
RUN mkdir -p /tmp/grepo
ENV GOPATH /tmp/grepo
ENV GOPROXY goproxy.cn
WORKDIR /tmp/workspace

RUN mkdir OnPremHeartBeat
RUN cd OnPremHeartBeat & git clone https://github.com/model-collapse/OnPremHeartBeat.git && cd OnPremHeartBeat && go build -o OnPremHeartBeat
RUN cp /tmp/workspace/OnPremHeartBeat/OnPremHeartBeat/OnPremHeartBeat . && chmod +x OnPremHeartBeat

CMD ./OnPremHeartBeat --interval=10 --root_name="heartbeats" --zk_addr="n1.onprem.ai:2181"