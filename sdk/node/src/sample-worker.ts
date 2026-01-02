import { connectNATS } from "./bus";
import { startWorker } from "./worker";

async function main() {
  const nc = await connectNATS({ url: "nats://127.0.0.1:4222" });
  await startWorker({
    nc,
    subject: "job.echo",
    senderId: "worker-echo-node",
    handler: async (req) => {
      return {
        jobId: req.jobId,
        status: "JOB_STATUS_SUCCEEDED",
        resultPtr: `redis://res/${req.jobId}`,
        workerId: "worker-echo-node",
      };
    },
  });
  console.log("echo worker listening on job.echo");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
