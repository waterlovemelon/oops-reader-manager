import { Card, Col, Row, Statistic } from 'antd';

export function DashboardPage() {
  return (
    <Row gutter={16}>
      <Col span={6}><Card><Statistic title="用户" value={0} /></Card></Col>
      <Col span={6}><Card><Statistic title="帖子" value={0} /></Card></Col>
      <Col span={6}><Card><Statistic title="书籍" value={0} /></Card></Col>
      <Col span={6}><Card><Statistic title="待处理" value={0} /></Card></Col>
    </Row>
  );
}
