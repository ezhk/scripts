#!/usr/bin/perl

use warnings;
use strict;

use MongoDB 0.702.2;
use Getopt::Long;
use feature qw{say};


sub _say_and_exit {
	my ($msg) = @_;

	unless (defined $msg) {
		say 'CRITICAL - empty message';
		exit 1;
	}

	say $msg;
	exit;
}


my %h_RS_myState_map = (
	0 => 'STARTUP',
	1 => 'PRIMARY',
	2 => 'SECONDARY',
	3 => 'RECOVERING',
	4 => 'FATAL',
	5 => 'STARTUP2',
	6 => 'UNKNOWN',
	7 => 'ARBITER',
	8 => 'DOWN',
	9 => 'ROLLBACK',
	10 => 'SHUNNED',
);

our %context = (
	'hostname'	=> 'localhost',
	'port'		=> 27017,
	'user'		=> undef,
	'password'	=> undef,
	'help'		=> 0,
);

GetOptions(
	'h|hostname=s'	=> \$context{'hostname'},
	'p|port=i'	=> \$context{'port'},
	'u|user=s'	=> \$context{'user'},
	'ps|password=s'	=> \$context{'password'},
	'help'		=> \$context{'help'},
);


if ($context{'help'}) {
	print q{Synopsis
	perl mongodb_rs.pl [options]

Desription
	Script return rs.status() in shortened form: "status - message",
e.g. "CRITICAL - MongoClient call failed, maybe cannot connect?".
Possible statuses and their descriptions:
CRITICAL:
	- couldn't connect to mongodb;
	- cannot change database to "admin";
	- db.runCommand({replSetGetStatus:1}) exec failure;
	- replSetGetStatus have no true "ok", possible error;
	- cannot find or not defined fileds: members, myState, set;
	- rs member have status FATAL, UNKNOWN, DOWN, ROLLBACK or STUNNED;
	- PRIMARY not found;
WARNING:
	- rs member have status STARTUP, STARTUP2 or RECOREVING;
	- cannot find or not defined fields for members: health, state, stateStr;
	- not epmty errmsg for RS-members (exception: "syncing to: ");
OK:
	- all okay ;)

Options and default values:
	-h, --hostname		localhost
	-p, --port		27017
	-u, --user		undefined
	-ps, --password		undefined
	--help
};

	exit;
}


my $client;
my %h_conn = (host => $context{'hostname'} . ':' . $context{'port'});
$h_conn{username} = $context{'user'} if (defined $context{'user'});
$h_conn{password} = $context{'password'} if (defined $context{'password'});

eval { $client = MongoDB::MongoClient->new(%h_conn) };
_say_and_exit('CRITICAL - MongoClient call failed, maybe cannot connect?')
	if ($@ or !$client);

my $database = $client->get_database("admin");
_say_and_exit('CRITICAL - get_database admin call failed')
	unless ($database);

my $rs_status = $database->run_command({replSetGetStatus => 1});
_say_and_exit('CRITICAL - error in call db.runCommand({replSetGetStatus:1})')
	unless ($rs_status and ref($rs_status) eq 'HASH');

_say_and_exit('CRITICAL - error message: ' . $rs_status->{errmsg})
	if (exists $rs_status->{errmsg} and $rs_status->{errmsg});

# validate hash data
unless (
	exists $rs_status->{ok}		and
	exists $rs_status->{members}	and
	exists $rs_status->{myState}	and
	exists $rs_status->{set}
) {
	_say_and_exit('CRITICAL - invalid some data (e.g. "ok", "members", "myState", "set"); check rs.status() manually');
}

_say_and_exit('CRITICAL - output not ok, check rs.status() manually')
	unless ($rs_status->{ok});


my @warnings;

# error states:
# 4  - FATAL
# 6  - UNKNOWN
# 8  - DOWN
# 9  - ROLLBACK
# 10 - STUNNED
if (defined $rs_status->{myState}) {
	if (
		$rs_status->{myState} == 4	or
		$rs_status->{myState} == 6	or
		$rs_status->{myState} >= 8
	) {
		_say_and_exit('CRITICAL - RS err state: '  . $h_RS_myState_map{$rs_status->{myState}});
	}

	# Initialization states:
	# 0 - STARTUP
	# 3 - RECOREVING
	# 5 - STARTUP2
	if (
		$rs_status->{myState} == 0	or
		$rs_status->{myState} == 3	or
		$rs_status->{myState} == 5
	) {
		push(@warnings, 'RS init state: ' . $h_RS_myState_map{$rs_status->{myState}});
	}
} else {
	_say_and_exit('CRITICAL - RS state not defined');
}

_say_and_exit('CRITICAL - empty RS members list')
	unless (scalar @{$rs_status->{members}});


my $primary_found_flag = 0;

# Search broken members
for my $member ( @{$rs_status->{members}} ) {
	# check that member have no PRIMARY/SECONDARY/ARBITER state
	if (defined $member->{state}) {
		# set true flag if found Primary server
		if ($member->{state} == 1) { $primary_found_flag++; }

		# SECONDARY/ARBITER - normally state
		elsif (
			scalar(@warnings) < 1 and
			$member->{state} != 2 and
			$member->{state} != 7
		) {
			push(@warnings, $member->{name} . ' state: ' . $member->{stateStr});
			next;
		}
	}

	# added warnings RS if empty array with warnings on this server
	unless ( scalar(@warnings) ) {
		unless (exists $member->{name}) {
			push(@warnings, 'cannot get name for RS member...?');
			next;
		} 

		# message "syncing to" isn't error: jira.mongodb.org/browse/SERVER-7099
		if (exists $member->{errmsg} and $member->{errmsg} and index($member->{errmsg}, 'syncing to: ') < 0) {
			push(@warnings, $member->{name} . ' errmsg: ' . $member->{errmsg});
			next;
		}

		unless (
			exists $member->{health}	or
			exists $member->{state}		or
			exists $member->{stateStr}
		) {
			push(@warnings, 'cannot define ' . $member->{name} . ' health status and state');
			next;
		}

		unless ($member->{health}) {
			push(@warnings, 'false health status for ' . $member->{name});
			next;
		}
	}
}

_say_and_exit('CRITICAL - not found PRIMARY')
	unless ($primary_found_flag);

_say_and_exit('WARNING - ' . join('; ', @warnings))
	if (scalar @warnings);

say 'OK - I seem, that all is working';
