# coding=utf-8

"""
Simple diamond collector, that read data from files in JSON format.
Format might be present as key-value pair, e.g.
{
   "metric_one" : 1,
   "metric_two" : 5,
}
"""

import diamond.collector
import os
import json


class FileJSONCollector(diamond.collector.Collector):

    def get_default_config_help(self):
        config_help = super(FileJSONCollector, self).get_default_config_help()
        config_help.update({
            'files': "list of json files, that contains metrics key=value",
            'paths':  "rename metric, by default: 'filejson,'",
        })
        self.log.info(config_help)
        return config_help

    def get_default_config(self):
        config = super(FileJSONCollector, self).get_default_config()
        config.update({
            'files': [],
            'paths': ['filejson'],
        })
        return config

    def _process_config(self):
        internal_data = {
            'files': list(),
            'paths': list(),
        }

        for options in ['files', 'paths']:
            config_files = self.config[options]
            if isinstance(config_files, basestring):
                internal_data[options] = map(lambda x: x.strip(),
                                             config_files.split(','))
            elif isinstance(config_files, list):
                internal_data[options] = config_files

        return (internal_data['files'],
                internal_data['paths'], )

    def collect(self):
        (files, paths) = self._process_config()

        for idx, fn in enumerate(files):
            if not os.access(fn, os.R_OK):
                self.log.error('No such file: %s' % fn)
                continue

            paths_idx = idx if idx < len(paths) else 0
            self.config['path'] = paths[paths_idx]

            with open(fn, 'r') as fh:
                try:
                    json_data = json.load(fh)
                except:
                    self.log.error('JSON load error %s' % fn)
                    continue

                for key, value in json_data.iteritems():
                    try:
                        self.publish(key, value)
                    except Exception as err:
                        self.log.error('publish error %s' % err)
        return True
