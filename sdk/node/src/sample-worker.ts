import { connectNATS } from "./bus";
import { startWorker } from "./worker";

async function main() {
  const nc = await connectNATS({ url: "nats://127.0.0.1:4222" });
  await startWorker({
    nc,
    subject: "job.echo",
    handler: async (req) => {
      return {
        job_id: req.job_id,
        status: "JOB_STATUS_SUCCEEDED",
        result_ptr: `redis://res/${req.job_id}`,
        worker_id: "worker-echo-node",
      };
    },
  });
  console.log("echo worker listening on job.echo");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
