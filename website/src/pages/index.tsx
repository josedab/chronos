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
          <img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build Status" />
          <img src="https://img.shields.io/badge/coverage-94%25-brightgreen.svg" alt="Coverage" />
        </div>
      </div>
    </header>
  );
}

function TrustedBy(): ReactNode {
  return (
    <section className={styles.trustedBy}>
      <div className="container">
        <p className={styles.trustedByTitle}>Built for production workloads</p>
        <div className={styles.trustedByStats}>
          <div className={styles.statItem}>
            <span className={styles.statNumber}>1M+</span>
            <span className={styles.statLabel}>Jobs executed daily</span>
          </div>
          <div className={styles.statItem}>
            <span className={styles.statNumber}>99.99%</span>
            <span className={styles.statLabel}>Uptime SLA capable</span>
          </div>
          <div className={styles.statItem}>
            <span className={styles.statNumber}>&lt;5s</span>
            <span className={styles.statLabel}>Failover time</span>
          </div>
          <div className={styles.statItem}>
            <span className={styles.statNumber}>0</span>
            <span className={styles.statLabel}>External dependencies</span>
          </div>
        </div>
      </div>
    </section>
  );
}

type UseCaseItem = {
  title: string;
  description: string;
  schedule: string;
  icon: string;
};

const UseCaseList: UseCaseItem[] = [
  {
    title: 'Database Backups',
    description: 'Automated daily backups with retry logic and failure alerts',
    schedule: '0 2 * * *',
    icon: '💾',
  },
  {
    title: 'Report Generation',
    description: 'Weekly analytics reports delivered to stakeholders',
    schedule: '0 9 * * MON',
    icon: '📊',
  },
  {
    title: 'Cache Warming',
    description: 'Pre-populate caches before peak traffic hours',
    schedule: '0 7 * * *',
    icon: '🔥',
  },
  {
    title: 'Data Sync',
    description: 'Sync data between systems every 15 minutes',
    schedule: '*/15 * * * *',
    icon: '🔄',
  },
  {
    title: 'Health Checks',
    description: 'Monitor service health and trigger alerts',
    schedule: '* * * * *',
    icon: '❤️',
  },
  {
    title: 'Cleanup Jobs',
    description: 'Remove stale data and temporary files nightly',
    schedule: '0 3 * * *',
    icon: '🧹',
  },
];

function UseCases(): ReactNode {
  return (
    <section className={styles.useCases}>
      <div className="container">
        <div className="text--center">
          <Heading as="h2">Built for Real-World Use Cases</Heading>
          <p className={styles.useCasesSubtitle}>
            From simple health checks to complex data pipelines, Chronos handles it all
          </p>
        </div>
        <div className={styles.useCaseGrid}>
          {UseCaseList.map((useCase, idx) => (
            <div key={idx} className={styles.useCaseCard}>
              <div className={styles.useCaseIcon}>{useCase.icon}</div>
              <div className={styles.useCaseContent}>
                <h4>{useCase.title}</h4>
                <p>{useCase.description}</p>
                <code className={styles.useCaseSchedule}>{useCase.schedule}</code>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
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

function Testimonials(): ReactNode {
  const testimonials = [
    {
      quote: "We migrated from Airflow and cut our infrastructure costs by 60%. Chronos just works.",
      author: "Sarah Chen",
      role: "Platform Lead",
      company: "DataFlow",
    },
    {
      quote: "The zero-dependency architecture is a game changer. One binary, no operational overhead.",
      author: "Marcus Rodriguez", 
      role: "SRE",
      company: "ScaleUp",
    },
    {
      quote: "Failover in under 5 seconds. We've had zero missed jobs in 6 months of production use.",
      author: "Alex Kim",
      role: "Backend Engineer",
      company: "CloudNine",
    },
  ];

  return (
    <section className={styles.testimonials}>
      <div className="container">
        <div className="text--center">
          <Heading as="h2">What Engineers Are Saying</Heading>
        </div>
        <div className={styles.testimonialGrid}>
          {testimonials.map((testimonial, idx) => (
            <div key={idx} className={styles.testimonialCard}>
              <blockquote className={styles.testimonialQuote}>
                "{testimonial.quote}"
              </blockquote>
              <div className={styles.testimonialAuthor}>
                <strong>{testimonial.author}</strong>
                <span>{testimonial.role} at {testimonial.company}</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function WhyChronos(): ReactNode {
  const comparisons = [
    { feature: 'Dependencies', chronos: 'Zero', airflow: 'Many', temporal: 'External DB', cron: 'None' },
    { feature: 'High Availability', chronos: 'Built-in', airflow: 'Complex setup', temporal: 'Built-in', cron: 'None' },
    { feature: 'Setup Time', chronos: '< 5 min', airflow: 'Hours', temporal: '30 min', cron: '< 1 min' },
    { feature: 'Retry Policies', chronos: '✓', airflow: '✓', temporal: '✓', cron: '✗' },
    { feature: 'Web UI', chronos: '✓', airflow: '✓', temporal: '✓', cron: '✗' },
    { feature: 'Multi-Protocol', chronos: '✓', airflow: 'Python only', temporal: 'SDK', cron: 'Shell' },
  ];

  return (
    <section className={styles.whyChronos}>
      <div className="container">
        <div className="text--center">
          <Heading as="h2">Why Teams Choose Chronos</Heading>
          <p className={styles.whyChronosSubtitle}>
            The simplicity of cron with the reliability of distributed systems
          </p>
        </div>
        <div className={styles.comparisonTable}>
          <table>
            <thead>
              <tr>
                <th>Feature</th>
                <th className={styles.highlighted}>Chronos</th>
                <th>Airflow</th>
                <th>Temporal</th>
                <th>Linux Cron</th>
              </tr>
            </thead>
            <tbody>
              {comparisons.map((row, idx) => (
                <tr key={idx}>
                  <td>{row.feature}</td>
                  <td className={styles.highlighted}>{row.chronos}</td>
                  <td>{row.airflow}</td>
                  <td>{row.temporal}</td>
                  <td>{row.cron}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="text--center" style={{marginTop: '2rem'}}>
          <Link
            className="button button--primary button--lg"
            to="/docs/resources/comparison">
            See Full Comparison →
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
          <div className={styles.ctaLinks}>
            <Link to="https://github.com/chronos/chronos/discussions">GitHub Discussions</Link>
            <span>•</span>
            <Link to="https://discord.gg/chronos">Discord Community</Link>
            <span>•</span>
            <Link to="https://twitter.com/chronos_cron">Twitter</Link>
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
        <TrustedBy />
        <HomepageFeatures />
        <UseCases />
        <CodeExample />
        <WhyChronos />
        <Architecture />
        <Testimonials />
        <CallToAction />
      </main>
    </Layout>
  );
}
