import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import CodeBlock from '@theme/CodeBlock';

import styles from './index.module.css';

function HomepageHeader() {
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className={styles.heroTitle}>
          Distributed Cron.<br />
          Zero Dependencies.<br />
          Bulletproof Reliability.
        </Heading>
        <p className={styles.heroSubtitle}>
          Schedule jobs across your infrastructure with at-least-once execution guarantees.
          Single binary. Raft consensus. Production-ready in minutes.
        </p>
        
        <div className={styles.installBox}>
          <code>curl -sSL https://get.chronos.dev | sh</code>
        </div>
        
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started/quickstart">
            Get Started →
          </Link>
          <Link
            className="button button--outline button--secondary button--lg"
            href="https://github.com/chronos/chronos">
            View on GitHub
          </Link>
        </div>

        <div className={styles.badges}>
          <img src="https://img.shields.io/badge/go-1.22+-blue.svg" alt="Go Version" />
          <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" />
          <img src="https://img.shields.io/github/stars/chronos/chronos?style=social" alt="GitHub Stars" />
        </div>
      </div>
    </header>
  );
}

type FeatureItem = {
  title: string;
  icon: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Zero Dependencies',
    icon: '📦',
    description: (
      <>
        Single binary with embedded BadgerDB storage. No external databases, 
        message queues, or coordination services required. Deploy anywhere.
      </>
    ),
  },
  {
    title: 'Distributed by Design',
    icon: '🔄',
    description: (
      <>
        Built on HashiCorp Raft for leader election and state replication. 
        Automatic failover in under 5 seconds. True high availability.
      </>
    ),
  },
  {
    title: 'At-Least-Once Execution',
    icon: '✓',
    description: (
      <>
        Jobs run even during node failures. Configurable retry policies with 
        exponential backoff. Never miss a scheduled execution.
      </>
    ),
  },
  {
    title: 'Multi-Protocol Dispatch',
    icon: '🌐',
    description: (
      <>
        HTTP webhooks, gRPC, Kafka, NATS, and RabbitMQ. Trigger any service 
        in any language. Built-in circuit breakers and timeouts.
      </>
    ),
  },
  {
    title: 'Observable',
    icon: '📊',
    description: (
      <>
        Prometheus metrics out of the box. Structured JSON logging. 
        OpenTelemetry tracing. Web UI for job management.
      </>
    ),
  },
  {
    title: 'Enterprise Ready',
    icon: '🏢',
    description: (
      <>
        RBAC, policy-as-code governance, secret injection from Vault and cloud 
        providers. Cross-region federation for global deployments.
      </>
    ),
  },
];

function Feature({title, icon, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4', styles.feature)}>
      <div className={styles.featureCard}>
        <div className={styles.featureIcon}>{icon}</div>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

function CodeExample(): ReactNode {
  const createJobCode = `curl -X POST http://localhost:8080/api/v1/jobs \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "daily-backup",
    "schedule": "0 2 * * *",
    "webhook": {
      "url": "https://api.example.com/backup",
      "method": "POST"
    },
    "retry_policy": {
      "max_attempts": 3,
      "initial_interval": "1s"
    }
  }'`;

  return (
    <section className={styles.codeExample}>
      <div className="container">
        <div className="row">
          <div className="col col--6">
            <Heading as="h2">Create a Job in Seconds</Heading>
            <p>
              Define your job with a simple JSON payload. Chronos handles scheduling, 
              retries, monitoring, and failover automatically.
            </p>
            <ul className={styles.featureList}>
              <li>✓ Standard cron expressions + @every syntax</li>
              <li>✓ Timezone support (IANA)</li>
              <li>✓ Configurable retry policies</li>
              <li>✓ Concurrency controls (allow/forbid/replace)</li>
              <li>✓ Custom headers and authentication</li>
            </ul>
            <Link
              className="button button--primary button--lg"
              to="/docs/getting-started/first-job">
              Learn More →
            </Link>
          </div>
          <div className="col col--6">
            <CodeBlock language="bash" title="Create a scheduled job">
              {createJobCode}
            </CodeBlock>
          </div>
        </div>
      </div>
    </section>
  );
}

function Architecture(): ReactNode {
  return (
    <section className={styles.architecture}>
      <div className="container">
        <div className="text--center">
          <Heading as="h2">Battle-Tested Architecture</Heading>
          <p className={styles.architectureSubtitle}>
            Chronos uses the same consensus algorithm that powers HashiCorp Consul and Vault
          </p>
        </div>
        <div className={styles.architectureDiagram}>
          <pre className={styles.asciiDiagram}>
{`┌──────────────────────────────────────────────────┐
│                  CHRONOS CLUSTER                  │
│                                                   │
│   ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │
│   │   Node 1    │ │   Node 2    │ │   Node 3   │ │
│   │  (Leader)   │ │ (Follower)  │ │ (Follower) │ │
│   │             │ │             │ │            │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │Scheduler│ │ │ │Scheduler│ │ │ │Scheduler│ │ │
│   │ │ (active)│ │ │ │(standby)│ │ │ │(standby)│ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │  Raft   │◄┼─┼─│  Raft   │◄┼─┼─│  Raft  │ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │BadgerDB │ │ │ │BadgerDB │ │ │ │BadgerDB│ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   └─────────────┘ └─────────────┘ └────────────┘ │
└──────────────────────────────────────────────────┘
                        │
                        ▼
              ┌─────────────────┐
              │ Target Services │
              │  (HTTP/gRPC/MQ) │
              └─────────────────┘`}
          </pre>
        </div>
        <div className="text--center">
          <Link
            className="button button--secondary button--lg"
            to="/docs/core-concepts/architecture">
            Explore Architecture →
          </Link>
        </div>
      </div>
    </section>
  );
}

function CallToAction(): ReactNode {
  return (
    <section className={styles.cta}>
      <div className="container">
        <div className="text--center">
          <Heading as="h2">Ready to Get Started?</Heading>
          <p>
            Deploy Chronos in your infrastructure in under 5 minutes.
            Run standalone or as a highly-available cluster.
          </p>
          <div className={styles.ctaButtons}>
            <Link
              className="button button--primary button--lg"
              to="/docs/getting-started/quickstart">
              Quick Start Guide
            </Link>
            <Link
              className="button button--secondary button--lg"
              to="/docs/guides/kubernetes">
              Deploy on Kubernetes
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Distributed Cron System"
      description="Chronos is a distributed cron system that provides reliable job scheduling without operational complexity. Zero dependencies, Raft consensus, at-least-once execution.">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <CodeExample />
        <Architecture />
        <CallToAction />
      </main>
    </Layout>
  );
}
