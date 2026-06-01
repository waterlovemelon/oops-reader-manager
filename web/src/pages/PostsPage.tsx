import { SearchOutlined } from '@ant-design/icons';
import { Button, Card, Input, Modal, Select, Space, Table, Tag, message } from 'antd';
import { useEffect, useState } from 'react';
import { Thread, Comment, listThreads, updateThreadStatus, listComments, updateCommentStatus } from '../api/community';

const STATUS_COLORS: Record<string, string> = {
  active: 'green',
  hidden: 'orange',
  locked: 'blue',
  deleted: 'red',
};

export function PostsPage() {
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('');

  const fetchThreads = () => {
    setLoading(true);
    listThreads(search || undefined, statusFilter ? undefined : undefined, 20)
      .then((res) => {
        setThreads(res.data);
        setTotal(res.pagination.total);
      })
      .catch(() => message.error('加载失败'))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchThreads(); }, [page, statusFilter]);

  const handleSearch = () => {
    setPage(1);
    fetchThreads();
  };

  const handleThreadStatus = async (id: string, newStatus: string) => {
    try {
      await updateThreadStatus(id, newStatus);
      message.success('状态已更新');
      fetchThreads();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '操作失败');
    }
  };

  const expandedRowRender = (thread: Thread) => {
    return <CommentsTable threadId={thread.id} />;
  };

  return (
    <Card title="帖子管理">
      <Space style={{ marginBottom: 16 }}>
        <Input
          placeholder="搜索标题/内容"
          prefix={<SearchOutlined />}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onPressEnter={handleSearch}
          style={{ width: 240 }}
          allowClear
        />
        <Select
          placeholder="状态筛选"
          value={statusFilter || undefined}
          onChange={(v) => { setStatusFilter(v ?? ''); setPage(1); }}
          allowClear
          style={{ width: 120 }}
          options={[
            { value: 'active', label: '正常' },
            { value: 'hidden', label: '已隐藏' },
            { value: 'locked', label: '已锁定' },
          ]}
        />
        <Button onClick={handleSearch}>搜索</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={threads}
        expandable={{ expandedRowRender, rowExpandable: (r) => r.comment_count > 0 }}
        pagination={{
          current: page,
          pageSize: 20,
          total,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        columns={[
          { title: '标题', dataIndex: 'title', ellipsis: true },
          { title: '版块', dataIndex: 'board_id', width: 80 },
          { title: '作者', dataIndex: 'author_id', width: 100 },
          { title: '评论', dataIndex: 'comment_count', width: 70 },
          {
            title: '状态', dataIndex: 'status', width: 100,
            render: (v: string) => <Tag color={STATUS_COLORS[v] ?? 'default'}>{v}</Tag>,
          },
          { title: '创建时间', dataIndex: 'created_at', width: 170 },
          {
            title: '操作', width: 200,
            render: (_: unknown, record: Thread) => (
              <Space size="small">
                {record.status === 'active' && (
                  <>
                    <Button size="small" onClick={() => handleThreadStatus(record.id, 'hidden')}>隐藏</Button>
                    <Button size="small" onClick={() => handleThreadStatus(record.id, 'locked')}>锁定</Button>
                  </>
                )}
                {(record.status === 'hidden' || record.status === 'locked') && (
                  <Button size="small" type="primary" onClick={() => handleThreadStatus(record.id, 'active')}>恢复</Button>
                )}
                {record.status !== 'deleted' && (
                  <Button size="small" danger onClick={() => {
                    Modal.confirm({
                      title: '确认删除？',
                      content: `确定要删除帖子《${record.title}》吗？`,
                      onOk: () => handleThreadStatus(record.id, 'deleted'),
                    });
                  }}>删除</Button>
                )}
              </Space>
            ),
          },
        ]}
      />
    </Card>
  );
}

function CommentsTable({ threadId }: { threadId: string }) {
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);

  const fetchComments = () => {
    setLoading(true);
    listComments(threadId, page)
      .then((res) => {
        setComments(res.data);
        setTotal(res.pagination.total);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchComments(); }, [page]);

  const handleCommentStatus = async (id: string, newStatus: string) => {
    try {
      await updateCommentStatus(id, newStatus);
      message.success('评论状态已更新');
      fetchComments();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '操作失败');
    }
  };

  return (
    <Table
      rowKey="id"
      loading={loading}
      dataSource={comments}
      size="small"
      pagination={{ current: page, pageSize: 10, total, onChange: setPage }}
      columns={[
        { title: '作者', dataIndex: 'author_id', width: 100 },
        { title: '内容', dataIndex: 'body', ellipsis: true },
        {
          title: '状态', dataIndex: 'status', width: 80,
          render: (v: string) => <Tag color={STATUS_COLORS[v] ?? 'default'}>{v}</Tag>,
        },
        { title: '时间', dataIndex: 'created_at', width: 170 },
        {
          title: '操作', width: 150,
          render: (_: unknown, record: Comment) => (
            <Space size="small">
              {record.status === 'active' && (
                <Button size="small" onClick={() => handleCommentStatus(record.id, 'hidden')}>隐藏</Button>
              )}
              {record.status === 'hidden' && (
                <Button size="small" type="primary" onClick={() => handleCommentStatus(record.id, 'active')}>恢复</Button>
              )}
              {record.status !== 'deleted' && (
                <Button size="small" danger onClick={() => handleCommentStatus(record.id, 'deleted')}>删除</Button>
              )}
            </Space>
          ),
        },
      ]}
    />
  );
}
