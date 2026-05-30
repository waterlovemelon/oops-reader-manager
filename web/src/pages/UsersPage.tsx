import { Card, Table, Tag } from 'antd';
import { useEffect, useState } from 'react';
import { User, listUsers } from '../api/users';

export function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    listUsers()
      .then((res) => setUsers(res.data))
      .finally(() => setLoading(false));
  }, []);

  return (
    <Card title="用户管理">
      <Table
        rowKey="id"
        loading={loading}
        dataSource={users}
        columns={[
          { title: 'ID', dataIndex: 'id' },
          { title: '邮箱', dataIndex: 'email' },
          { title: '昵称', dataIndex: 'display_name' },
          { title: '状态', dataIndex: 'status', render: (value: string) => <Tag>{value}</Tag> },
          { title: '创建时间', dataIndex: 'created_at' },
        ]}
      />
    </Card>
  );
}
