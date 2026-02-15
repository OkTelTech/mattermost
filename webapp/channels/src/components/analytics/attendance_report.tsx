// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { FormattedMessage, defineMessages, useIntl } from 'react-intl';
import { CellContext, ColumnDef, PaginationState, SortingState, getCoreRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from '@tanstack/react-table';

import { Client4 } from 'mattermost-redux/client';

import { AdminConsoleListTable, LoadingStates, PAGE_SIZES } from 'components/admin_console/list_table';
import type { TableMeta } from 'components/admin_console/list_table';
import AdminHeader from 'components/widgets/admin_console/admin_header';
import Avatar from 'components/widgets/users/avatar';
import LoadingScreen from 'components/loading_screen';
import { imageURLForUser } from 'utils/utils';

// Import filter components
import { AttendanceReportSearch, AttendanceReportDateFilter, AttendanceReportTeamFilter } from './attendance_report_filters';
import StatisticCount from './statistic_count';

import './attendance_report.scss';

// Types
type AttendanceStats = {
    from: string;
    to: string;
    total_checked_in: number;
    total_working: number;
    total_on_break: number;
    total_checked_out: number;
    total_on_leave: number;
    total_late_arrivals: number;
    total_early_departures: number;
    pending_requests: number;
};

type AttendanceEntry = {
    date: string;
    check_in: string;
    check_out?: string;
    status: string;
};

type LeaveEntry = {
    type: string;
    dates: string[];
    reason: string;
    expected_time?: string;
    status: string;
};

type UserReport = {
    id: string; // Required for AdminConsoleListTable
    user_id: string;
    username: string;
    days_worked: number;
    days_leave: number;
    late_arrivals: number;
    early_departures: number;
    attendance: AttendanceEntry[];
    leave_requests: LeaveEntry[];
};

type AttendanceReport = {
    from: string;
    to: string;
    users: Omit<UserReport, 'id'>[];
};

const messages = defineMessages({
    title: { id: 'analytics.attendance.title', defaultMessage: 'Attendance Report' },
    checkedIn: { id: 'analytics.attendance.checkedIn', defaultMessage: 'Checked In' },
    onLeave: { id: 'analytics.attendance.onLeave', defaultMessage: 'On Leave' },
    lateArrivals: { id: 'analytics.attendance.lateArrivals', defaultMessage: 'Late Arrivals' },
    earlyDepartures: { id: 'analytics.attendance.earlyDepartures', defaultMessage: 'Early Departures' },
    pendingRequests: { id: 'analytics.attendance.pendingRequests', defaultMessage: 'Pending Requests' },
    username: { id: 'analytics.attendance.username', defaultMessage: 'Username' },
    daysWorked: { id: 'analytics.attendance.daysWorked', defaultMessage: 'Days Worked' },
    daysLeave: { id: 'analytics.attendance.daysLeave', defaultMessage: 'Days Leave' },
    noData: { id: 'analytics.attendance.noData', defaultMessage: 'No attendance data found for this date range.' },
    date: { id: 'analytics.attendance.date', defaultMessage: 'Date' },
    checkIn: { id: 'analytics.attendance.checkIn', defaultMessage: 'Check In' },
    checkOut: { id: 'analytics.attendance.checkOut', defaultMessage: 'Check Out' },
    status: { id: 'analytics.attendance.status', defaultMessage: 'Status' },
    type: { id: 'analytics.attendance.type', defaultMessage: 'Type' },
    dates: { id: 'analytics.attendance.dates', defaultMessage: 'Dates' },
    reason: { id: 'analytics.attendance.reason', defaultMessage: 'Reason' },
    back: { id: 'analytics.attendance.back', defaultMessage: 'Back to all users' },
    attendanceDetail: { id: 'analytics.attendance.attendanceDetail', defaultMessage: 'Attendance' },
    leaveDetail: { id: 'analytics.attendance.leaveDetail', defaultMessage: 'Leave Requests' },
    userDetail: { id: 'analytics.attendance.userDetail', defaultMessage: 'Detail for {username}' },
    from: { id: 'analytics.attendance.from', defaultMessage: 'From' },
    to: { id: 'analytics.attendance.to', defaultMessage: 'To' },
    searchPlaceholder: { id: 'analytics.attendance.searchPlaceholder', defaultMessage: 'Search by username...' },
});

function formatDateStr(date: Date): string {
    return date.toISOString().slice(0, 10);
}


function apiFetch(url: string): Promise<Response> {
    const headers: Record<string, string> = { Accept: 'application/json' };
    const token = Client4.getToken();
    if (token) {
        headers.Authorization = `Bearer ${token}`;
    }
    return fetch(url, { headers, credentials: 'include' });
}

const statusBadge = (status: string) => {
    let color = '#999';
    if (status === 'approved' || status === 'completed') {
        color = '#339970';
    } else if (status === 'pending' || status === 'working') {
        color = '#f5a623';
    } else if (status === 'rejected') {
        color = '#d24b4e';
    } else if (status === 'break') {
        color = '#1e88e5';
    }
    return (
        <span
            style={{
                padding: '2px 8px',
                borderRadius: '10px',
                backgroundColor: color,
                color: '#fff',
                fontSize: '12px',
            }}
        >
            {status}
        </span>
    );
};

// User detail panel
const UserDetailPanel: React.FC<{ user: UserReport }> = ({ user }) => (
    <div>




        {/* Summary */}
        <div className='grid-statistics'>
            <StatisticCount
                title={<FormattedMessage {...messages.daysWorked} />}
                icon='fa-briefcase'
                count={user.days_worked}
            />
            <StatisticCount
                title={<FormattedMessage {...messages.daysLeave} />}
                icon='fa-calendar-times-o'
                count={user.days_leave}
            />
            <StatisticCount
                title={<FormattedMessage {...messages.lateArrivals} />}
                icon='fa-clock-o'
                count={user.late_arrivals}
                status={user.late_arrivals > 0 ? 'warning' : undefined}
            />
            <StatisticCount
                title={<FormattedMessage {...messages.earlyDepartures} />}
                icon='fa-sign-out'
                count={user.early_departures}
            />
        </div>

        {/* Attendance table */}
        {user.attendance && user.attendance.length > 0 && (
            <div style={{ marginTop: '20px' }}>
                <h5><FormattedMessage {...messages.attendanceDetail} /></h5>
                <div className='total-count recent-active-users'>
                    <table className='table'>
                        <thead>
                            <tr>
                                <th><FormattedMessage {...messages.date} /></th>
                                <th><FormattedMessage {...messages.checkIn} /></th>
                                <th><FormattedMessage {...messages.checkOut} /></th>
                                <th><FormattedMessage {...messages.status} /></th>
                            </tr>
                        </thead>
                        <tbody>
                            {user.attendance.map((entry) => (
                                <tr key={entry.date}>
                                    <td>{entry.date}</td>
                                    <td>{entry.check_in}</td>
                                    <td>{entry.check_out || '-'}</td>
                                    <td>{statusBadge(entry.status)}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>
        )}

        {/* Leave requests table */}
        {user.leave_requests && user.leave_requests.length > 0 && (
            <div style={{ marginTop: '20px' }}>
                <h5><FormattedMessage {...messages.leaveDetail} /></h5>
                <div className='total-count recent-active-users'>
                    <table className='table'>
                        <thead>
                            <tr>
                                <th><FormattedMessage {...messages.type} /></th>
                                <th><FormattedMessage {...messages.dates} /></th>
                                <th><FormattedMessage {...messages.reason} /></th>
                                <th><FormattedMessage {...messages.status} /></th>
                            </tr>
                        </thead>
                        <tbody>
                            {user.leave_requests.map((req, idx) => (
                                <tr key={idx}>
                                    <td>{req.type}</td>
                                    <td>{req.dates.join(', ')}</td>
                                    <td>{req.reason}</td>
                                    <td>{statusBadge(req.status)}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>
        )}
    </div>
);

const AttendanceReportPage: React.FC = () => {
    const { formatMessage } = useIntl();
    const [selectedMonth, setSelectedMonth] = useState(() => {
        const now = new Date();
        return now.toISOString().slice(0, 7); // YYYY-MM
    });

    const [stats, setStats] = useState<AttendanceStats | null>(null);
    const [report, setReport] = useState<AttendanceReport | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [selectedUser, setSelectedUser] = useState<UserReport | null>(null);

    // Filter states
    const [searchTerm, setSearchTerm] = useState('');
    const [filterTeam, setFilterTeam] = useState('');
    const [filterTeamLabel, setFilterTeamLabel] = useState('');

    // Table states
    const [sorting, setSorting] = useState<SortingState>([]);
    const [pagination, setPagination] = useState<PaginationState>({ pageIndex: 0, pageSize: PAGE_SIZES[0] });

    const filteredUsers: UserReport[] = useMemo(() => {
        if (!report?.users) {
            return [];
        }
        let users = report.users.map(u => ({ ...u, id: u.user_id }));

        if (searchTerm.trim()) {
            const term = searchTerm.trim().toLowerCase();
            users = users.filter((u) => u.username.toLowerCase().includes(term));
        }

        return users;
    }, [report, searchTerm]);

    const fetchData = useCallback(async () => {
        setLoading(true);
        setError('');
        setSelectedUser(null);

        // Calculate from/to based on selectedMonth
        const [yearStr, monthStr] = selectedMonth.split('-');
        const year = parseInt(yearStr, 10);
        const month = parseInt(monthStr, 10);

        // First day: YYYY-MM-01
        const from = `${selectedMonth}-01`;

        // Last day calculation
        const lastDay = new Date(year, month, 0).getDate();
        const to = `${selectedMonth}-${lastDay}`;

        const base = Client4.getBaseRoute();
        let query = `from=${from}&to=${to}`;

        if (filterTeam && filterTeam !== 'teams_filter_for_all_teams') {
            query += `&team_id=${filterTeam}`;
        }

        try {
            const [statsRes, reportRes] = await Promise.all([
                apiFetch(`${base}/bot-service/attendance/stats?${query}`),
                apiFetch(`${base}/bot-service/attendance/report?${query}`),
            ]);

            if (!statsRes.ok) {
                throw new Error(`Stats API error: ${statsRes.status}`);
            }
            if (!reportRes.ok) {
                throw new Error(`Report API error: ${reportRes.status}`);
            }

            const [statsData, reportData] = await Promise.all([
                statsRes.json() as Promise<AttendanceStats>,
                reportRes.json() as Promise<AttendanceReport>,
            ]);

            setStats(statsData);
            setReport(reportData);
        } catch (err: any) {
            setError(err.message || 'Failed to load attendance data');
        } finally {
            setLoading(false);
        }
    }, [selectedMonth, filterTeam]);

    useEffect(() => {
        fetchData();
    }, [fetchData]);

    // Define columns
    const columns = useMemo<ColumnDef<UserReport, any>[]>(() => [
        {
            id: 'username',
            accessorKey: 'username',
            header: formatMessage(messages.username),
            cell: (info: CellContext<UserReport, unknown>) => (
                <div className='TeamList_nameColumn'>
                    <div className='TeamList__lowerOpacity' style={{ marginRight: '12px' }}>
                        <Avatar
                            size='sm'
                            url={imageURLForUser(info.row.original.user_id)}
                            username={info.getValue() as string}
                        />
                    </div>
                    <div className='TeamList_nameText'>
                        <span
                            onClick={() => setSelectedUser(info.row.original)}
                            style={{ cursor: 'pointer', fontWeight: 'bold' }}
                        >
                            {info.getValue() as string}
                        </span>
                    </div>
                </div>
            ),
        },
        {
            id: 'days_worked',
            accessorKey: 'days_worked',
            header: formatMessage(messages.daysWorked),
            cell: (info) => info.getValue(),
        },
        {
            id: 'days_leave',
            accessorKey: 'days_leave',
            header: formatMessage(messages.daysLeave),
            cell: (info) => info.getValue(),
        },
        {
            id: 'late_arrivals',
            accessorKey: 'late_arrivals',
            header: formatMessage(messages.lateArrivals),
            cell: (info) => info.getValue(),
        },
        {
            id: 'early_departures',
            accessorKey: 'early_departures',
            header: formatMessage(messages.earlyDepartures),
            cell: (info) => info.getValue(),
        },
    ], [formatMessage]);

    // Table instance
    const table = useReactTable({
        data: filteredUsers,
        columns,
        state: {
            sorting,
            pagination,
        },
        onSortingChange: setSorting,
        onPaginationChange: setPagination,
        getCoreRowModel: getCoreRowModel(),
        getSortedRowModel: getSortedRowModel(),
        getPaginationRowModel: getPaginationRowModel(),
        meta: {
            tableId: 'attendanceReportTable',
            tableCaption: formatMessage(messages.title),
            loadingState: loading ? LoadingStates.Loading : LoadingStates.Loaded,
            onRowClick: (id) => {
                const user = filteredUsers.find(u => u.id === id);
                if (user) {
                    setSelectedUser(user);
                }
            },
        } as TableMeta,
    });

    if (loading && !report) {
        // Show loading screen initially
        return <LoadingScreen />;
    }

    return (
        <div className='wrapper--fixed team_statistics'>
            <AdminHeader withBackButton={Boolean(selectedUser)}>
                {selectedUser ? (
                    <div>
                        <div
                            onClick={() => setSelectedUser(null)}
                            className='fa fa-angle-left back'
                            style={{ cursor: 'pointer' }}
                        />
                        <FormattedMessage
                            {...messages.userDetail}
                            values={{ username: selectedUser.username }}
                        />
                    </div>
                ) : (
                    <FormattedMessage {...messages.title} />
                )}
            </AdminHeader>

            <div className='admin-console__wrapper'>
                <div className='admin-console__content'>
                    {error && (
                        <div className='banner banner--error'>
                            <div className='banner__content'>
                                <span>{error}</span>
                            </div>
                        </div>
                    )}

                    {selectedUser ? (
                        <UserDetailPanel
                            user={selectedUser}
                        />
                    ) : (
                        <>
                            {/* Stats cards */}
                            {stats && (
                                <div className='grid-statistics' style={{ marginBottom: '20px' }}>
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.checkedIn} />}
                                        icon='fa-sign-in'
                                        count={stats.total_checked_in}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.onLeave} />}
                                        icon='fa-calendar-times-o'
                                        count={stats.total_on_leave}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.lateArrivals} />}
                                        icon='fa-clock-o'
                                        count={stats.total_late_arrivals}
                                        status={stats.total_late_arrivals > 0 ? 'warning' : undefined}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.earlyDepartures} />}
                                        icon='fa-sign-out'
                                        count={stats.total_early_departures}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.pendingRequests} />}
                                        icon='fa-hourglass-half'
                                        count={stats.pending_requests}
                                        status={stats.pending_requests > 0 ? 'warning' : undefined}
                                    />
                                </div>
                            )}

                            <div className='admin-console__container ignore-marking'>
                                <div className='admin-console__filters-rows'>
                                    <AttendanceReportSearch
                                        term={searchTerm}
                                        onSearch={setSearchTerm}
                                    />
                                    <AttendanceReportTeamFilter
                                        filterTeam={filterTeam}
                                        filterTeamLabel={filterTeamLabel}
                                        onChange={(id, label) => {
                                            setFilterTeam(id);
                                            setFilterTeamLabel(label || '');
                                        }}
                                    />
                                </div>
                                <div className='admin-console__filters-rows'>
                                    <AttendanceReportDateFilter
                                        month={selectedMonth}
                                        onChange={setSelectedMonth}
                                    />

                                </div>
                                <AdminConsoleListTable
                                    table={table}
                                />
                            </div>
                        </>
                    )}
                </div>
            </div>
        </div>
    );
};

export default AttendanceReportPage;
