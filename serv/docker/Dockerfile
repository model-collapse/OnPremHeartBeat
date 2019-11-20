FROM modelcollapse/raspbian-go:strech

RUN mkdir -p /tmp/workspace
RUN mkdir -p /tmp/grepo
ENV GOPATH /tmp/grepo
ENV GOPROXY goproxy.cn
WORKDIR /tmp/workspace

RUN git clone https://github.com/model-collapse/OnPremHeartBeat.git && cd OnPremHeartBeat/serv && go build -o ophb-serv
RUN cp OnPremHeartBeat/serv/ophb-serv . && chmod +x ophb-serv

CMD ./ophb-serv --root_name="heartbeats" --zk_addr="n1.onprem.ai:2181"
