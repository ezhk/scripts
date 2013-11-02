#!/usr/bin/env perl

use warnings;
use strict;

use YAML qw{LoadFile};

use lib::abs qw{../modules};
use API::YandexDNS;
use Log qw{_print_it};


our $config = '/etc/dynamic_ya_dns.yml';


my $yml_data;
eval {
	require YAML;
	$yml_data = YAML::LoadFile($config);
};
if ($@) {
	_print_it("cannot read YAML config: " . $@);
	exit 1;
}

unless (keys %{$yml_data}) {
	_print_it("YAML config empty?");
	exit 1;
}


for my $domain_name (keys %{$yml_data}) {
	unless (exists $yml_data->{$domain_name}->{'token'}) {
		_print_it("token id must be defined for domain '$domain_name'");
		next;
	}

	$yml_data->{$domain_name}->{"v4"} //= "auto";
	$yml_data->{$domain_name}->{"v6"} //= "auto";

	my $yndx = API::YandexDNS->new(
		$yml_data->{$domain_name}->{'token'},
		undef,
		{
			'ttl' => '180',
		}
	);
	unless ($yndx) {
		_print_it('YandexDNS->new return false');
		exit 1;
	}

	my $exists_record = $yndx->_check_exists_record($domain_name);

	for my $addr_type ( qw{v4 v6} ) {
		$yml_data->{$domain_name}->{$addr_type} //= "auto";
		next if ($yml_data->{$domain_name}->{$addr_type} eq "none");

		my $ip_addr = $yml_data->{$domain_name}->{$addr_type};
		if ($yml_data->{$domain_name}->{$addr_type} eq "auto") {
			my $sock;
			if ($addr_type eq "v4") {
				eval {
					require IO::Socket::INET;
					$sock = IO::Socket::INET->new(
						'Proto' => 'udp',
						'PeerAddr' => 'a.root-servers.net',
						'PeerPort' => '53'
					);
				};
			} elsif ($addr_type eq "v6") {
				eval {
					require IO::Socket::INET6;
					$sock = IO::Socket::INET6->new(
						'Proto' => 'udp',
						'PeerAddr' => 'a.root-servers.net',
						'PeerPort' => '53'
					);
				};
			}
			if ($@) {
				_print_it("IO::Socket call return error: " . $@);
				next;
			}
			unless ($sock && $sock->sockhost) {
				_print_it("cannot get $addr_type addr type");
				next;
			}

			$ip_addr = $sock->sockhost;
		}

		my $record_type;
		if ($addr_type eq "v4") { $record_type = "A"; }
		elsif ( $addr_type eq "v6" ) { $record_type = "AAAA"; }

		if ($exists_record) {
			my $exist_record_flag = 0;
			for my $id (keys %{$exists_record}) {
				if (
					exists $exists_record->{$id}->{'type'}			&&
					exists $exists_record->{$id}->{'content'}		&&
					uc($exists_record->{$id}->{'type'}) eq $record_type	&&
					$exists_record->{$id}->{'content'} eq $ip_addr
				) {
					$exist_record_flag++;
					last;
				}
			}

			if ($exist_record_flag) { next; }
		}

		unless ($yndx->set_record($domain_name, $record_type, $ip_addr) ) {
			_print_it('cannot set record: ' . $domain_name . ' IN ' . $record_type . ' ' . $ip_addr);
		}
	}
}

exit;
