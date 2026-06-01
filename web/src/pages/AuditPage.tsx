import { Card, DatePicker, Input, Select, Space, Table, Tag, Button } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useEffect, useState } from 'react';
import { AuditEntry, listAuditLogs } from '../api/audit';
import type { Dayjs } from 'dayjs';

const RESOURCE_LABELS: Record<string, string> = {
  book: '书籍',
  user: '用户',
  thread: '帖子',
  comment: '评论',
};

const ACTION_LABELS: Record<string, string> = {
  status_change: '状态变更',
  update: '更新',
  publish: '发布',
  freeze: '冻结',
  hide: '隐藏',
};

export function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [admin, setAdmin] = useState('');
  const [resourceType, setResourceType] = useState('');
  const [dateRange, setDateRange] = useState<[Dayjs | null, Dayjs | null]>([null, null]);

  const fetchLogs = () => {
    setLoading(true);
    listAuditLogs({
      admin: admin || undefined,
      resource_type: resourceType || undefined,
      start: dateRange[0]?.format('YYYY-MM-DD'),
      end: dateRange[1]?.format('YYYY-MM-DD'),
      page,
    })
      .then((res) => {
        setEntries(res.data);
        setTotal(res.pagination.total);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchLogs(); }, [page]);

  const handleSearch = () => {
    setPage(1);
    fetchLogs();
  };

  return (
    <Card title="审计日志">
      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder="管理员用户名"
          prefix={<SearchOutlined />}
          value={admin}
          onChange={(e) => setAdmin(e.target.value)}
          onPressEnter={handleSearch}
          style={{ width: 160 }}
          allowClear
        />
        <Select
          placeholder="资源类型"
          value={resourceType || undefined}
          onChange={(v) => { setResourceType(v ?? ''); setPage(1); }}
          allowClear
          style={{ width: 120 }}
          options={[
            { value: 'book', label: '书籍' },
            { value: 'user', label: '用户' },
            { value: 'thread', label: '帖子' },
            { value: 'comment', label: '评论' },
          ]}
        />
        <DatePicker.RangePicker
          onChange={(dates) => setDateRange(dates as [Dayjs | null, Dayjs | null])}
        />
        <Button onClick={handleSearch}>搜索</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={entries}
        pagination={{
          current: page,
          pageSize: 50,
          total,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        columns={[
          { title: '时间', dataIndex: 'created_at', width: 170 },
          { title: '管理员', dataIndex: 'admin_username', width: 100 },
          {
            title: '操作', dataIndex: 'action', width: 100,
            render: (v: string) => <Tag>{ACTION_LABELS[v] ?? v}</Tag>,
          },
          {
            title: '资源类型', dataIndex: 'resource_type', width: 80,
            render: (v: string) => RESOURCE_LABELS[v] ?? v,
          },
          { title: '资源ID', dataIndex: 'resource_id', ellipsis: true },
          { title: 'IP', dataIndex: 'ip_address', width: 130 },
        ]}
      />
    </Card>
  );
}
