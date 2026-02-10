// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {FormattedMessage, defineMessages, useIntl} from 'react-intl';

import {Client4} from 'mattermost-redux/client';

import LoadingScreen from 'components/loading_screen';
import AdminHeader from 'components/widgets/admin_console/admin_header';

import StatisticCount from './statistic_count';

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
    users: UserReport[];
};

const messages = defineMessages({
    title: {id: 'analytics.attendance.title', defaultMessage: 'Attendance Report'},
    checkedIn: {id: 'analytics.attendance.checkedIn', defaultMessage: 'Checked In'},
    onLeave: {id: 'analytics.attendance.onLeave', defaultMessage: 'On Leave'},
    lateArrivals: {id: 'analytics.attendance.lateArrivals', defaultMessage: 'Late Arrivals'},
    earlyDepartures: {id: 'analytics.attendance.earlyDepartures', defaultMessage: 'Early Departures'},
    pendingRequests: {id: 'analytics.attendance.pendingRequests', defaultMessage: 'Pending Requests'},
    username: {id: 'analytics.attendance.username', defaultMessage: 'Username'},
    daysWorked: {id: 'analytics.attendance.daysWorked', defaultMessage: 'Days Worked'},
    daysLeave: {id: 'analytics.attendance.daysLeave', defaultMessage: 'Days Leave'},
    noData: {id: 'analytics.attendance.noData', defaultMessage: 'No attendance data found for this date range.'},
    date: {id: 'analytics.attendance.date', defaultMessage: 'Date'},
    checkIn: {id: 'analytics.attendance.checkIn', defaultMessage: 'Check In'},
    checkOut: {id: 'analytics.attendance.checkOut', defaultMessage: 'Check Out'},
    status: {id: 'analytics.attendance.status', defaultMessage: 'Status'},
    type: {id: 'analytics.attendance.type', defaultMessage: 'Type'},
    dates: {id: 'analytics.attendance.dates', defaultMessage: 'Dates'},
    reason: {id: 'analytics.attendance.reason', defaultMessage: 'Reason'},
    back: {id: 'analytics.attendance.back', defaultMessage: 'Back to all users'},
    attendanceDetail: {id: 'analytics.attendance.attendanceDetail', defaultMessage: 'Attendance'},
    leaveDetail: {id: 'analytics.attendance.leaveDetail', defaultMessage: 'Leave Requests'},
    userDetail: {id: 'analytics.attendance.userDetail', defaultMessage: 'Detail for {username}'},
    from: {id: 'analytics.attendance.from', defaultMessage: 'From'},
    to: {id: 'analytics.attendance.to', defaultMessage: 'To'},
    searchPlaceholder: {id: 'analytics.attendance.searchPlaceholder', defaultMessage: 'Search by username...'},
});

function formatDateStr(date: Date): string {
    return date.toISOString().slice(0, 10);
}

function getDefaultFrom(): string {
    const d = new Date();
    d.setDate(1);
    return formatDateStr(d);
}

function getDefaultTo(): string {
    return formatDateStr(new Date());
}

function apiFetch(url: string): Promise<Response> {
    const headers: Record<string, string> = {Accept: 'application/json'};
    const token = Client4.getToken();
    if (token) {
        headers.Authorization = `Bearer ${token}`;
    }
    return fetch(url, {headers, credentials: 'include'});
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
const UserDetailPanel: React.FC<{user: UserReport; onBack: () => void}> = ({user, onBack}) => (
    <div>
        <button
            className='btn btn-link'
            onClick={onBack}
            style={{marginBottom: '12px', padding: 0}}
        >
            <i className='fa fa-arrow-left' style={{marginRight: '6px'}}/>
            <FormattedMessage {...messages.back}/>
        </button>

        <h4>
            <FormattedMessage
                {...messages.userDetail}
                values={{username: user.username}}
            />
        </h4>

        {/* Summary */}
        <div className='grid-statistics'>
            <StatisticCount
                title={<FormattedMessage {...messages.daysWorked}/>}
                icon='fa-briefcase'
                count={user.days_worked}
            />
            <StatisticCount
                title={<FormattedMessage {...messages.daysLeave}/>}
                icon='fa-calendar-times-o'
                count={user.days_leave}
            />
            <StatisticCount
                title={<FormattedMessage {...messages.lateArrivals}/>}
                icon='fa-clock-o'
                count={user.late_arrivals}
                status={user.late_arrivals > 0 ? 'warning' : undefined}
            />
            <StatisticCount
                title={<FormattedMessage {...messages.earlyDepartures}/>}
                icon='fa-sign-out'
                count={user.early_departures}
            />
        </div>

        {/* Attendance table */}
        {user.attendance && user.attendance.length > 0 && (
            <div style={{marginTop: '20px'}}>
                <h5><FormattedMessage {...messages.attendanceDetail}/></h5>
                <div className='total-count recent-active-users'>
                    <table className='table'>
                        <thead>
                            <tr>
                                <th><FormattedMessage {...messages.date}/></th>
                                <th><FormattedMessage {...messages.checkIn}/></th>
                                <th><FormattedMessage {...messages.checkOut}/></th>
                                <th><FormattedMessage {...messages.status}/></th>
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
            <div style={{marginTop: '20px'}}>
                <h5><FormattedMessage {...messages.leaveDetail}/></h5>
                <div className='total-count recent-active-users'>
                    <table className='table'>
                        <thead>
                            <tr>
                                <th><FormattedMessage {...messages.type}/></th>
                                <th><FormattedMessage {...messages.dates}/></th>
                                <th><FormattedMessage {...messages.reason}/></th>
                                <th><FormattedMessage {...messages.status}/></th>
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
    const [from, setFrom] = useState(getDefaultFrom);
    const [to, setTo] = useState(getDefaultTo);
    const [stats, setStats] = useState<AttendanceStats | null>(null);
    const [report, setReport] = useState<AttendanceReport | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [selectedUser, setSelectedUser] = useState<UserReport | null>(null);
    const [searchTerm, setSearchTerm] = useState('');
    const intl = useIntl();

    const filteredUsers = useMemo(() => {
        if (!report?.users) {
            return [];
        }
        if (!searchTerm.trim()) {
            return report.users;
        }
        const term = searchTerm.trim().toLowerCase();
        return report.users.filter((u) => u.username.toLowerCase().includes(term));
    }, [report, searchTerm]);

    const fetchData = useCallback(async () => {
        setLoading(true);
        setError('');
        setSelectedUser(null);

        const base = Client4.getBaseRoute();

        try {
            const [statsRes, reportRes] = await Promise.all([
                apiFetch(`${base}/bot-service/attendance/stats?from=${from}&to=${to}`),
                apiFetch(`${base}/bot-service/attendance/report?from=${from}&to=${to}`),
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
    }, [from, to]);

    useEffect(() => {
        fetchData();
    }, [fetchData]);

    if (loading) {
        return <LoadingScreen/>;
    }

    return (
        <div className='wrapper--fixed team_statistics'>
            <AdminHeader>
                <FormattedMessage {...messages.title}/>
            </AdminHeader>

            <div className='admin-console__wrapper'>
                <div className='admin-console__content'>
                    {/* Date range filter */}
                    <div
                        className='banner'
                        style={{display: 'flex', gap: '16px', alignItems: 'center', padding: '12px 16px'}}
                    >
                        <label style={{display: 'flex', alignItems: 'center', gap: '8px', margin: 0}}>
                            <FormattedMessage {...messages.from}/>
                            <input
                                type='date'
                                className='form-control'
                                value={from}
                                onChange={(e) => setFrom(e.target.value)}
                                style={{width: 'auto'}}
                            />
                        </label>
                        <label style={{display: 'flex', alignItems: 'center', gap: '8px', margin: 0}}>
                            <FormattedMessage {...messages.to}/>
                            <input
                                type='date'
                                className='form-control'
                                value={to}
                                onChange={(e) => setTo(e.target.value)}
                                style={{width: 'auto'}}
                            />
                        </label>
                        <input
                            type='text'
                            className='form-control'
                            placeholder={intl.formatMessage(messages.searchPlaceholder)}
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            style={{width: '220px', marginLeft: 'auto'}}
                        />
                    </div>

                    {error && (
                        <div className='banner banner--error'>
                            <div className='banner__content'>
                                <span>{error}</span>
                            </div>
                        </div>
                    )}

                    {/* User detail view */}
                    {selectedUser ? (
                        <UserDetailPanel
                            user={selectedUser}
                            onBack={() => setSelectedUser(null)}
                        />
                    ) : (
                        <>
                            {/* Stats cards */}
                            {stats && (
                                <div className='grid-statistics'>
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.checkedIn}/>}
                                        icon='fa-sign-in'
                                        count={stats.total_checked_in}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.onLeave}/>}
                                        icon='fa-calendar-times-o'
                                        count={stats.total_on_leave}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.lateArrivals}/>}
                                        icon='fa-clock-o'
                                        count={stats.total_late_arrivals}
                                        status={stats.total_late_arrivals > 0 ? 'warning' : undefined}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.earlyDepartures}/>}
                                        icon='fa-sign-out'
                                        count={stats.total_early_departures}
                                    />
                                    <StatisticCount
                                        title={<FormattedMessage {...messages.pendingRequests}/>}
                                        icon='fa-hourglass-half'
                                        count={stats.pending_requests}
                                        status={stats.pending_requests > 0 ? 'warning' : undefined}
                                    />
                                </div>
                            )}

                            {/* Per-user report table */}
                            {filteredUsers.length > 0 ? (
                                <div className='col-sm-12' style={{marginTop: '20px'}}>
                                    <div className='total-count recent-active-users'>
                                        <table className='table'>
                                            <thead>
                                                <tr>
                                                    <th><FormattedMessage {...messages.username}/></th>
                                                    <th><FormattedMessage {...messages.daysWorked}/></th>
                                                    <th><FormattedMessage {...messages.daysLeave}/></th>
                                                    <th><FormattedMessage {...messages.lateArrivals}/></th>
                                                    <th><FormattedMessage {...messages.earlyDepartures}/></th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {filteredUsers.map((user) => (
                                                    <tr
                                                        key={user.user_id}
                                                        onClick={() => setSelectedUser(user)}
                                                        style={{cursor: 'pointer'}}
                                                    >
                                                        <td>
                                                            <a style={{cursor: 'pointer'}}>
                                                                {user.username}
                                                            </a>
                                                        </td>
                                                        <td>{user.days_worked}</td>
                                                        <td>{user.days_leave}</td>
                                                        <td>{user.late_arrivals}</td>
                                                        <td>{user.early_departures}</td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                </div>
                            ) : (
                                !error && (
                                    <div className='banner'>
                                        <div className='banner__content'>
                                            <FormattedMessage {...messages.noData}/>
                                        </div>
                                    </div>
                                )
                            )}
                        </>
                    )}
                </div>
            </div>
        </div>
    );
};

export default AttendanceReportPage;
