import { Client, StatusOK } from "k6/net/grpc";
import { check, sleep } from "k6";

const client = new Client();
client.load(
  ["../../", "../../internal/thirdparty/protobuf"],
  "pkg/protocol/feed-server/feed_server.proto"
);

export default () => {
  // only on the first iteration
  if (__ITER == 0) {
    // {feed_addr}
    client.connect("127.0.0.1:9510", { plaintext: true });
  }

  const data = {
    bizId: 0, // {biz_id}
    app_meta: { app: "{app}" },
    key: "{key}",
  };

  const params = {
    metadata: {
      "sidecar-meta": `{"bid": 0, "fpt": "xxx"}`,
      Authorization: "Bearer " + "{token}",
    },
  };
  const res = client.invoke("pbfs.Upstream/GetKvValue", data, params);

  check(res, {
    "status is OK": (r) => r && r.status === StatusOK,
  });

  // console.log(res.status, JSON.stringify(res.message));

  // client.close();
  sleep(Math.random() * 0.1);
};
