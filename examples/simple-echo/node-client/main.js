const { connectNATS, submitJob } = require("../../../sdk/node/dist");

async function main() {
  const nc = await connectNATS({ url: "nats://127.0.0.1:4222" });
  await submitJob(
    nc,
    {
      jobId: "job-echo-1",
      topic: "job.echo",
      contextPtr: "redis://ctx/job-echo-1",
    },
    "trace-1",
    "client-node"
  );
  await nc.drain();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
