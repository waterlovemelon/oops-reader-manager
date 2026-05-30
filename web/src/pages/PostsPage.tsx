import { Card, Table, Tag } from 'antd';
import { useEffect, useState } from 'react';
import { Thread, listThreads } from '../api/community';

export function PostsPage() {
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    listThreads()
      .then((res) => setThreads(res.data))
      .finally(() => setLoading(false));
  }, []);

  return (
    <Card title="帖子管理">
      <Table
        rowKey="id"
        loading={loading}
        dataSource={threads}
        columns={[
          { title: '标题', dataIndex: 'title' },
          { title: '版块', dataIndex: 'board_id' },
          { title: '作者', dataIndex: 'author_id' },
          { title: '评论', dataIndex: 'comment_count' },
          { title: '状态', dataIndex: 'status', render: (value: string) => <Tag>{value}</Tag> },
          { title: '创建时间', dataIndex: 'created_at' },
        ]}
      />
    </Card>
  );
}
