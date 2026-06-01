import { Card, Col, Row, Statistic, Spin } from 'antd';
import { useEffect, useState } from 'react';
import { DashboardSummary, getSummary } from '../api/dashboard';

export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    getSummary()
      .then((res) => setSummary(res.data))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  return (
    <Spin spinning={loading}>
      <Row gutter={16}>
        <Col span={6}>
          <Card><Statistic title="用户" value={summary?.total_users ?? 0} /></Card>
        </Col>
        <Col span={6}>
          <Card><Statistic title="帖子" value={summary?.total_threads ?? 0} /></Card>
        </Col>
        <Col span={6}>
          <Card><Statistic title="书籍" value={summary?.total_books ?? 0} /></Card>
        </Col>
        <Col span={6}>
          <Card><Statistic title="待处理" value={summary?.pending_review ?? 0} valueStyle={{ color: '#faad14' }} /></Card>
        </Col>
      </Row>
    </Spin>
  );
}
