---
seotitle: Start building backends using Encore.ts
seodesc: Learn how Encore.ts works, and get to know the powerful features that help you build cloud backend applications easier than ever before.
title: Encore.ts
subtitle: Use Encore.ts to build production-ready backend applications and distributed systems
toc: false
lang: ts
---

<div className="min-h-72 bg-blue p-8 relative overflow-hidden not-prose">
    <img className="absolute left-[55%] -mt-8 top-0 right-0 bottom-0 noshadow" src="/assets/img/dithered-clouds.png" />
    <div className="w-[75%] lg:w-[75%]">
        <h2 className="text-white lead-medium">Quick Start Guide</h2>
        <div className="body-small text-white mt-2">
            Dive right in and build your first Encore.ts application.
        </div>
        <a href="/docs/ts/quick-start">
            <Button className="mt-4" kind="primary" section="black">Get started</Button>
        </a>
    </div>
</div>

Encore.ts is a high-performance API framework that helps you build robust, type-safe applications. It provides a declarative approach to working with essential backend infrastructure like microservices, databases, queues, caches, cron jobs, and storage buckets.

The framework comes with a lot of built-in tooling for a smooth developer experience:

1. **Local Environment Management**: Automatically sets up and runs your local development environment and all local infrastructure.
2. **Enhanced Observability**: Comes with tools like a [Local Development Dashboard](/docs/ts/observability/dev-dash) and [tracing](/docs/ts/observability/tracing) for monitoring application behavior.
3. **Automatic Documentation**: Generates and maintains [up-to-date documentation](/docs/ts/observability/service-catalog) for APIs and services, and created [architecture diagrams](/docs/ts/observability/flow) for your system.

Optional: **DevOps Automation**: Encore provides an optional [Cloud Platform](/use-cases/devops-automation) for automating infrastructure provisioning and DevOps processes on AWS and GCP.

<div className="mt-6 grid grid-cols-2 gap-6 mobile:grid-cols-1 not-prose">
    <a className="block group relative no-brandient" target="_blank" href="https://www.youtube.com/watch?v=vvqTGfoXVsw">
        <div className="absolute inset-0 bg-black dark:bg-white -z-10" />
        <div
            className="min-h-full border border-black dark:border-white p-8 mobile:p-4 bg-white dark:bg-black transition-transform duration-100 ease-in-out group-active:-translate-x-2 group-active:-translate-y-2 group-hover:-translate-x-2 group-hover:-translate-y-2">
            <div className="flex items-center justify-between">
                <h3 className="body-small">Watch an intro video</h3>
                <img className="-mt-2 h-16 w-16 noshadow" src="/assets/icons/features/preview-envs.png" />
            </div>
            <div className="mt-2">
                Get to know the core concepts of Encore in this short video.
            </div>
        </div>
    </a>
    <a className="block group relative no-brandient" target="_blank" href="https://github.com/encoredev/examples">
        <div className="absolute inset-0 bg-black dark:bg-white -z-10" />
        <div
            className="min-h-full border border-black dark:border-white p-8 mobile:p-4 bg-white dark:bg-black transition-transform duration-100 ease-in-out group-active:-translate-x-2 group-active:-translate-y-2 group-hover:-translate-x-2 group-hover:-translate-y-2">
            <div className="flex items-center justify-between">
                <h3 className="body-small">Example apps</h3>
                <img className="-mt-2 h-16 w-16 noshadow" src="/assets/icons/features/flow.png" />
            </div>
            <div className="mt-2">
                Ready-made starter apps to inspire your development.
            </div>
        </div>
    </a>
    <a className="block group relative no-brandient" href="/discord">
        <div className="absolute inset-0 bg-black dark:bg-white -z-10" />
        <div
            className="min-h-full border border-black dark:border-white p-8 mobile:p-4 bg-white dark:bg-black transition-transform duration-100 ease-in-out group-active:-translate-x-2 group-active:-translate-y-2 group-hover:-translate-x-2 group-hover:-translate-y-2">
            <div className="flex items-center justify-between">
                <h3 className="body-small">Join Discord</h3>
                <div className="inline-flex w-16 h-16 items-center justify-center">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 -28.5 256 256">
                        <path fill="#111111"
                            d="M216.856339,16.5966031 C200.285002,8.84328665 182.566144,3.2084988 164.041564,0 C161.766523,4.11318106 159.108624,9.64549908 157.276099,14.0464379 C137.583995,11.0849896 118.072967,11.0849896 98.7430163,14.0464379 C96.9108417,9.64549908 94.1925838,4.11318106 91.8971895,0 C73.3526068,3.2084988 55.6133949,8.86399117 39.0420583,16.6376612 C5.61752293,67.146514 -3.4433191,116.400813 1.08711069,164.955721 C23.2560196,181.510915 44.7403634,191.567697 65.8621325,198.148576 C71.0772151,190.971126 75.7283628,183.341335 79.7352139,175.300261 C72.104019,172.400575 64.7949724,168.822202 57.8887866,164.667963 C59.7209612,163.310589 61.5131304,161.891452 63.2445898,160.431257 C105.36741,180.133187 151.134928,180.133187 192.754523,160.431257 C194.506336,161.891452 196.298154,163.310589 198.110326,164.667963 C191.183787,168.842556 183.854737,172.420929 176.223542,175.320965 C180.230393,183.341335 184.861538,190.991831 190.096624,198.16893 C211.238746,191.588051 232.743023,181.531619 254.911949,164.955721 C260.227747,108.668201 245.831087,59.8662432 216.856339,16.5966031 Z M85.4738752,135.09489 C72.8290281,135.09489 62.4592217,123.290155 62.4592217,108.914901 C62.4592217,94.5396472 72.607595,82.7145587 85.4738752,82.7145587 C98.3405064,82.7145587 108.709962,94.5189427 108.488529,108.914901 C108.508531,123.290155 98.3405064,135.09489 85.4738752,135.09489 Z M170.525237,135.09489 C157.88039,135.09489 147.510584,123.290155 147.510584,108.914901 C147.510584,94.5396472 157.658606,82.7145587 170.525237,82.7145587 C183.391518,82.7145587 193.761324,94.5189427 193.539891,108.914901 C193.539891,123.290155 183.391518,135.09489 170.525237,135.09489 Z" />
                    </svg>
                </div>
            </div>
            <div className="mt-2">
                Find answers, ask questions, and chat with other Encore developers.
            </div>
        </div>
    </a>
    <a className="block group relative no-brandient" href="https://github.com/encoredev/encore">
        <div className="absolute inset-0 bg-black dark:bg-white -z-10" />
        <div
            className="min-h-full border border-black dark:border-white p-8 mobile:p-4 bg-white dark:bg-black transition-transform duration-100 ease-in-out group-active:-translate-x-2 group-active:-translate-y-2 group-hover:-translate-x-2 group-hover:-translate-y-2">
            <div className="flex items-center justify-between">
                <h3 className="body-small">Star on GitHub</h3>
                <div className="inline-flex w-16 h-16 items-center justify-center">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 16 16" fill="#111111"
                        stroke="none">
                        <path fillRule="evenodd"
                            d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                    </svg>
                </div>
            </div>
            <div className="mt-2">
                Get involved and star Encore on GitHub.
            </div>
        </div>
    </a>
</div>
