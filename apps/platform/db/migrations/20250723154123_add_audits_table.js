exports.up = async function(knex) {
    await knex.schema.createTable('audits', (table) => {
        table.increments()
        table.integer('project_id')
            .unsigned()
            .notNullable()
            .references('id')
            .inTable('projects')
            .onDelete('CASCADE')
        table.integer('admin_id')
            .unsigned()
            .nullable()
            .references('id')
            .inTable('admins')
            .onDelete('CASCADE')
        table.string('item_type', 50).notNull()
        table.integer('item_id').unsigned().notNull()
        table.string('event', 50).notNull().index()
        table.json('object').nullable()
        table.json('object_changes').nullable()

        table.timestamp('created_at')
            .notNull()
            .defaultTo(knex.fn.now())
            .index()
        table.timestamp('updated_at')
            .notNull()
            .defaultTo(knex.fn.now())

        table.index(['item_type', 'item_id'])
    })
}

exports.down = async function(knex) {
    await knex.schema
        .dropTable('audits')
}
